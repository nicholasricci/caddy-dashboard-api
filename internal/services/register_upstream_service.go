package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrNoOperationalNodes  = errors.New("no operational nodes in discovery group")
	ErrNoReachableLeader   = errors.New("no reachable caddy node in discovery group")
	ErrInvalidRegisterDial = errors.New("invalid dial: provide dial or private_ip and port")
	ErrRegisterLockTimeout = errors.New("register upstream lock timeout")
)

type RegisterUpstreamService struct {
	caddy       *CaddyService
	nodes       *repository.NodeRepository
	discoveries *repository.DiscoveryRepository
	profiles    *repository.UpstreamProfileRepository
	db          *gorm.DB
	locksMu     sync.Mutex
	locks       map[uuid.UUID]*sync.Mutex
}

func NewRegisterUpstreamService(
	caddy *CaddyService,
	nodes *repository.NodeRepository,
	discoveries *repository.DiscoveryRepository,
	profiles *repository.UpstreamProfileRepository,
	db *gorm.DB,
) *RegisterUpstreamService {
	return &RegisterUpstreamService{
		caddy:       caddy,
		nodes:       nodes,
		discoveries: discoveries,
		profiles:    profiles,
		db:          db,
		locks:       make(map[uuid.UUID]*sync.Mutex),
	}
}

type RegisterUpstreamInput struct {
	DiscoveryConfigID uuid.UUID
	ConfigID          string
	Port              int
	PrivateIP         string
	Dial              string
	DryRun            bool
	RequestedBy       string
}

type RegisterUpstreamByProfileInput struct {
	ProfileID   uuid.UUID
	PrivateIP   string
	DryRun      bool
	RequestedBy string
}

type RegisterUpstreamTarget struct {
	ConfigID string
	Dial     string
}

type RegisterUpstreamResult struct {
	DiscoveryConfigID uuid.UUID
	SourceNodeID      uuid.UUID
	Dial              string
	Targets           []RegisterUpstreamTarget
	Changed           bool
	DryRun            bool
	Mutate            *caddysvc.MutateUpstreamsResponse
	Propagate         *caddysvc.PropagateResponse
}

func (s *RegisterUpstreamService) Register(ctx context.Context, in RegisterUpstreamInput) (*RegisterUpstreamResult, error) {
	if _, err := s.discoveries.GetByID(ctx, in.DiscoveryConfigID); err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	dial, err := buildRegisterDial(in.PrivateIP, in.Port, in.Dial)
	if err != nil {
		return nil, err
	}
	configID := strings.TrimSpace(in.ConfigID)
	if configID == "" {
		return nil, caddysvc.ErrInvalidMutationPayload
	}
	return s.registerTargets(ctx, in.DiscoveryConfigID, []RegisterUpstreamTarget{{
		ConfigID: configID,
		Dial:     dial,
	}}, in.DryRun, in.RequestedBy)
}

func (s *RegisterUpstreamService) GetProfile(ctx context.Context, profileID uuid.UUID) (*models.UpstreamProfile, error) {
	profile, err := s.profiles.GetByID(ctx, profileID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUpstreamProfileNotFound
		}
		return nil, err
	}
	return profile, nil
}

func (s *RegisterUpstreamService) RegisterByProfile(ctx context.Context, in RegisterUpstreamByProfileInput) (*RegisterUpstreamResult, error) {
	profile, err := s.GetProfile(ctx, in.ProfileID)
	if err != nil {
		return nil, err
	}
	ip := strings.TrimSpace(in.PrivateIP)
	if ip == "" {
		return nil, ErrInvalidRegisterDial
	}
	bindings, err := ParseBindings(profile.Bindings)
	if err != nil {
		return nil, err
	}
	targets := make([]RegisterUpstreamTarget, 0, len(bindings))
	for _, b := range bindings {
		targets = append(targets, RegisterUpstreamTarget{
			ConfigID: b.ConfigID,
			Dial:     fmt.Sprintf("%s:%d", ip, b.Port),
		})
	}
	result, err := s.registerTargets(ctx, profile.DiscoveryConfigID, targets, in.DryRun, in.RequestedBy)
	if err != nil {
		return nil, err
	}
	result.Dial = ""
	return result, nil
}

func (s *RegisterUpstreamService) registerTargets(
	ctx context.Context,
	discoveryConfigID uuid.UUID,
	targets []RegisterUpstreamTarget,
	dryRun bool,
	requestedBy string,
) (*RegisterUpstreamResult, error) {
	if len(targets) == 0 {
		return nil, caddysvc.ErrInvalidMutationPayload
	}
	var result *RegisterUpstreamResult
	err := s.withDiscoveryLock(ctx, discoveryConfigID, func() error {
		sourceID, err := s.selectReachableLeader(ctx, discoveryConfigID)
		if err != nil {
			return err
		}
		mutateTargets := make([]caddysvc.UpstreamMutationTarget, 0, len(targets))
		for _, t := range targets {
			mutateTargets = append(mutateTargets, caddysvc.UpstreamMutationTarget{
				ConfigID: t.ConfigID,
				AddDial:  t.Dial,
			})
		}
		mutateResp, err := s.caddy.MutateUpstreams(ctx, sourceID, caddysvc.MutateUpstreamsRequest{
			Targets: mutateTargets,
			DryRun:  dryRun,
		}, requestedBy)
		if err != nil {
			return err
		}
		out := &RegisterUpstreamResult{
			DiscoveryConfigID: discoveryConfigID,
			SourceNodeID:      sourceID,
			Targets:           append([]RegisterUpstreamTarget(nil), targets...),
			Changed:           mutateResp.Changed,
			DryRun:            dryRun,
			Mutate:            mutateResp,
		}
		if len(targets) == 1 {
			out.Dial = targets[0].Dial
		}
		if dryRun {
			result = out
			return nil
		}
		propagateResp, err := s.caddy.PropagateToDiscoveryPeers(ctx, sourceID, requestedBy)
		if err != nil {
			return err
		}
		out.Propagate = propagateResp
		result = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *RegisterUpstreamService) selectReachableLeader(ctx context.Context, discoveryConfigID uuid.UUID) (uuid.UUID, error) {
	nodes, err := s.nodes.ListByDiscoveryConfigID(ctx, discoveryConfigID)
	if err != nil {
		return uuid.Nil, err
	}
	candidates := make([]models.CaddyNode, 0, len(nodes))
	for _, node := range nodes {
		if node.EffectiveTransport() == models.TransportInventoryOnly {
			continue
		}
		candidates = append(candidates, node)
	}
	if len(candidates) == 0 {
		return uuid.Nil, ErrNoOperationalNodes
	}
	sort.Slice(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name)
	})
	var lastErr error
	for _, node := range candidates {
		if _, err := s.caddy.GetLiveConfig(ctx, node.ID); err == nil {
			return node.ID, nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return uuid.Nil, fmt.Errorf("%w: %v", ErrNoReachableLeader, lastErr)
	}
	return uuid.Nil, ErrNoReachableLeader
}

func buildRegisterDial(privateIP string, port int, dialOverride string) (string, error) {
	if dial := strings.TrimSpace(dialOverride); dial != "" {
		return dial, nil
	}
	ip := strings.TrimSpace(privateIP)
	if ip == "" || port <= 0 || port > 65535 {
		return "", ErrInvalidRegisterDial
	}
	return fmt.Sprintf("%s:%d", ip, port), nil
}

func (s *RegisterUpstreamService) withDiscoveryLock(ctx context.Context, discoveryID uuid.UUID, fn func() error) error {
	if s.db != nil {
		err := s.withMySQLLock(ctx, discoveryID, fn)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrRegisterLockTimeout) {
			return err
		}
	}
	lock := s.discoveryLock(discoveryID)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func (s *RegisterUpstreamService) withMySQLLock(ctx context.Context, discoveryID uuid.UUID, fn func() error) error {
	lockName := fmt.Sprintf("register_upstream:%s", discoveryID.String())
	if len(lockName) > 64 {
		lockName = lockName[:64]
	}
	var acquired int
	err := s.db.WithContext(ctx).Raw("SELECT GET_LOCK(?, ?)", lockName, 30).Scan(&acquired).Error
	if err != nil {
		return err
	}
	if acquired != 1 {
		return ErrRegisterLockTimeout
	}
	defer func() {
		var released int
		_ = s.db.WithContext(context.Background()).Raw("SELECT RELEASE_LOCK(?)", lockName).Scan(&released).Error
	}()
	return fn()
}

func (s *RegisterUpstreamService) discoveryLock(discoveryID uuid.UUID) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	lock, ok := s.locks[discoveryID]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[discoveryID] = lock
	}
	return lock
}
