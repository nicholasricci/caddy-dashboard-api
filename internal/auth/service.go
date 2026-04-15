package auth

import (
	"context"
	"errors"

	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRefreshInvalid     = errors.New("invalid refresh token")
)

type Service struct {
	cfg   config.AuthConfig
	users *repository.UserRepository
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func NewService(cfg config.AuthConfig, users *repository.UserRepository) *Service {
	return &Service{
		cfg:   cfg,
		users: users,
	}
}

func (s *Service) manager() *JWTManager {
	return NewJWTManager(s.cfg.JWTSecret, s.cfg.TokenTTLMinutes, s.cfg.RefreshTTLMinutes)
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
	access, refresh, err := s.manager().GeneratePair(user.Username, user.Role)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	return s.manager().ValidateAccessToken(token)
}
