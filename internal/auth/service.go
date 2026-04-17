package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRefreshInvalid     = errors.New("invalid refresh token")
)

type Service struct {
	cfg     config.AuthConfig
	users   *repository.UserRepository
	refresh *repository.RefreshTokenRepository
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func NewService(cfg config.AuthConfig, users *repository.UserRepository, refresh *repository.RefreshTokenRepository) *Service {
	return &Service{
		cfg:     cfg,
		users:   users,
		refresh: refresh,
	}
}

func (s *Service) manager() *JWTManager {
	return NewJWTManager(
		s.cfg.JWTSecret,
		s.cfg.TokenTTLMinutes,
		s.cfg.RefreshTTLMinutes,
		s.cfg.Issuer,
		s.cfg.Audience,
	)
}

func (s *Service) Login(ctx context.Context, username, password string) (*TokenPair, error) {
	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	access, refresh, err := s.manager().GeneratePair(username, user.Role)
	if err != nil {
		return nil, err
	}
	if err := s.storeRefreshToken(ctx, user.ID, refresh); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.manager().ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrRefreshInvalid
	}
	user, err := s.users.GetByUsername(ctx, claims.Username)
	if err != nil {
		return nil, ErrRefreshInvalid
	}
	if user.Role != claims.Role {
		return nil, ErrRefreshInvalid
	}
	hash := hashToken(refreshToken)
	stored, err := s.refresh.GetActiveByHash(ctx, hash)
	if err != nil {
		return nil, ErrRefreshInvalid
	}
	if stored.UserID != user.ID {
		return nil, ErrRefreshInvalid
	}
	access, refresh, err := s.manager().GeneratePair(user.Username, user.Role)
	if err != nil {
		return nil, err
	}
	if err := s.refresh.RevokeByHash(ctx, hash); err != nil {
		return nil, err
	}
	if err := s.storeRefreshToken(ctx, user.ID, refresh); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	return s.manager().ValidateAccessToken(token)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.refresh.RevokeByHash(ctx, hashToken(refreshToken))
}

func (s *Service) CleanupExpiredRefreshTokens(ctx context.Context) error {
	return s.refresh.CleanupExpired(ctx)
}

func (s *Service) storeRefreshToken(ctx context.Context, userID uuid.UUID, token string) error {
	claims, err := s.manager().ValidateRefreshToken(token)
	if err != nil {
		return err
	}
	return s.refresh.Create(ctx, &models.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(token),
		ExpiresAt: claims.ExpiresAt.Time.UTC(),
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
