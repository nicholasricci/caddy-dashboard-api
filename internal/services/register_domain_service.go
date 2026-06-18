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
	ErrInvalidRegisterDomains = errors.New("at least one domain is required")
)

type RegisterDomainService struct {
	caddy       *CaddyService
	nodes       *repository.NodeRepository
	discoveries *repository.DiscoveryRepository
	profiles    *repository.DomainProfileRepository
	db          *gorm.DB
	locksMu     sync.Mutex
	locks       map[uuid.UUID]*sync.Mutex
}

func NewRegisterDomainService(
	caddy *CaddyService,
	nodes *repository.NodeRepository,
	discoveries *repository.DiscoveryRepository,
	profiles *repository.DomainProfileRepository,
	db *gorm.DB,
) *RegisterDomainService {
	return &RegisterDomainService{
		caddy:       caddy,
		nodes:       nodes,
		discoveries: discoveries,
		profiles:    profiles,
		db:          db,
		locks:       make(map[uuid.UUID]*sync.Mutex),
	}
}

type RegisterDomainTLSInput struct {
	UpdateTLSPolicies bool
	DNSChallenge      *caddysvc.TLSDNSChallenge
}

type RegisterDomainInput struct {
	DiscoveryConfigID uuid.UUID
	ConfigID          string
	MatchIndexes      []int
	Domains           []string
	TLS               RegisterDomainTLSInput
	DryRun            bool
	RequestedBy       string
}

type RegisterDomainByProfileInput struct {
	ProfileID   uuid.UUID
	Domains     []string
	TLS         RegisterDomainTLSInput
	DryRun      bool
	RequestedBy string
}

type RegisterDomainTarget struct {
	ConfigID     string
	MatchIndexes []int
	Domains      []string
}

type RegisterDomainResult struct {
	DiscoveryConfigID uuid.UUID
	SourceNodeID      uuid.UUID
	Domains           []string
	Targets           []RegisterDomainTarget
	Changed           bool
	DryRun            bool
	Mutate            *caddysvc.MutateDomainsResponse
	Propagate         *caddysvc.PropagateResponse
}

func (s *RegisterDomainService) Register(ctx context.Context, in RegisterDomainInput) (*RegisterDomainResult, error) {
	if _, err := s.discoveries.GetByID(ctx, in.DiscoveryConfigID); err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	domains, err := normalizeRegisterDomains(in.Domains)
	if err != nil {
		return nil, err
	}
	configID := strings.TrimSpace(in.ConfigID)
	if configID == "" {
		return nil, caddysvc.ErrInvalidMutationPayload
	}
	indexes := in.MatchIndexes
	if len(indexes) == 0 {
		indexes = []int{0}
	}
	return s.registerTargets(ctx, in.DiscoveryConfigID, []RegisterDomainTarget{{
		ConfigID:     configID,
		MatchIndexes: indexes,
		Domains:      domains,
	}}, in.TLS, in.DryRun, in.RequestedBy)
}

func (s *RegisterDomainService) GetProfile(ctx context.Context, profileID uuid.UUID) (*models.DomainProfile, error) {
	profile, err := s.profiles.GetByID(ctx, profileID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDomainProfileNotFound
		}
		return nil, err
	}
	return profile, nil
}

func (s *RegisterDomainService) RegisterByProfile(ctx context.Context, in RegisterDomainByProfileInput) (*RegisterDomainResult, error) {
	profile, err := s.GetProfile(ctx, in.ProfileID)
	if err != nil {
		return nil, err
	}
	domains, err := normalizeRegisterDomains(in.Domains)
	if err != nil {
		return nil, err
	}
	bindings, err := ParseDomainBindings(profile.Bindings)
	if err != nil {
		return nil, err
	}
	targets := make([]RegisterDomainTarget, 0, len(bindings))
	for _, b := range bindings {
		targets = append(targets, RegisterDomainTarget{
			ConfigID:     b.ConfigID,
			MatchIndexes: b.MatchIndexes,
			Domains:      domains,
		})
	}
	result, err := s.registerTargets(ctx, profile.DiscoveryConfigID, targets, in.TLS, in.DryRun, in.RequestedBy)
	if err != nil {
		return nil, err
	}
	result.Domains = domains
	return result, nil
}

func (s *RegisterDomainService) registerTargets(
	ctx context.Context,
	discoveryConfigID uuid.UUID,
	targets []RegisterDomainTarget,
	tls RegisterDomainTLSInput,
	dryRun bool,
	requestedBy string,
) (*RegisterDomainResult, error) {
	if len(targets) == 0 {
		return nil, caddysvc.ErrInvalidMutationPayload
	}
	var result *RegisterDomainResult
	err := s.withDiscoveryLock(ctx, discoveryConfigID, func() error {
		sourceID, err := s.selectReachableLeader(ctx, discoveryConfigID)
		if err != nil {
			return err
		}
		mutateTargets := make([]caddysvc.DomainMutationTarget, 0, len(targets))
		allDomains := make([]string, 0)
		for _, t := range targets {
			mutateTargets = append(mutateTargets, caddysvc.DomainMutationTarget{
				ConfigID:     t.ConfigID,
				MatchIndexes: t.MatchIndexes,
				AddDomains:   t.Domains,
			})
			allDomains = append(allDomains, t.Domains...)
		}
		mutateResp, err := s.caddy.MutateDomains(ctx, sourceID, caddysvc.MutateDomainsRequest{
			Targets:           mutateTargets,
			UpdateTLSPolicies: tls.UpdateTLSPolicies,
			TLSDNSChallenge:   tls.DNSChallenge,
			DryRun:            dryRun,
		}, requestedBy)
		if err != nil {
			return err
		}
		out := &RegisterDomainResult{
			DiscoveryConfigID: discoveryConfigID,
			SourceNodeID:      sourceID,
			Targets:           append([]RegisterDomainTarget(nil), targets...),
			Changed:           mutateResp.Changed,
			DryRun:            dryRun,
			Mutate:            mutateResp,
		}
		if len(targets) == 1 {
			out.Domains = targets[0].Domains
		} else {
			out.Domains = uniqueSortedStrings(allDomains)
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

func (s *RegisterDomainService) selectReachableLeader(ctx context.Context, discoveryConfigID uuid.UUID) (uuid.UUID, error) {
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

func normalizeRegisterDomains(domains []string) ([]string, error) {
	if len(domains) == 0 {
		return nil, ErrInvalidRegisterDomains
	}
	seen := make(map[string]struct{}, len(domains))
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, ErrInvalidRegisterDomains
	}
	sort.Strings(out)
	return out, nil
}

func uniqueSortedStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		seen[s] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func (s *RegisterDomainService) withDiscoveryLock(ctx context.Context, discoveryID uuid.UUID, fn func() error) error {
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

func (s *RegisterDomainService) withMySQLLock(ctx context.Context, discoveryID uuid.UUID, fn func() error) error {
	lockName := fmt.Sprintf("register_domain:%s", discoveryID.String())
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

func (s *RegisterDomainService) discoveryLock(discoveryID uuid.UUID) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	lock, ok := s.locks[discoveryID]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[discoveryID] = lock
	}
	return lock
}
