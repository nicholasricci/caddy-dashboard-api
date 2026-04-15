package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameTaken    = errors.New("username already taken")
	ErrUsernameRequired = errors.New("username is required")
	ErrCannotDeleteSelf = errors.New("cannot delete your own account")
	ErrLastAdmin        = errors.New("cannot remove the last admin user")
	ErrInvalidRole      = errors.New("invalid role")
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.repo.List(ctx)
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

type CreateUserInput struct {
	Username string
	Password string
	Role     string
}

func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*models.User, error) {
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = models.RoleUser
	}
	if role != models.RoleAdmin && role != models.RoleUser {
		return nil, ErrInvalidRole
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &models.User{
		Username:     strings.TrimSpace(in.Username),
		PasswordHash: string(hash),
		Role:         role,
	}
	if u.Username == "" {
		return nil, ErrUsernameRequired
	}
	if _, err := s.repo.GetByUsername(ctx, u.Username); err == nil {
		return nil, ErrUsernameTaken
	} else if err != nil && !repository.IsNotFound(err) {
		return nil, err
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

type UpdateUserInput struct {
	Username *string
	Password *string
	Role     *string
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, in UpdateUserInput) (*models.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if in.Username != nil && strings.TrimSpace(*in.Username) != "" {
		other, err := s.repo.GetByUsername(ctx, strings.TrimSpace(*in.Username))
		if err == nil && other.ID != u.ID {
			return nil, ErrUsernameTaken
		}
		if err != nil && !repository.IsNotFound(err) {
			return nil, err
		}
		u.Username = strings.TrimSpace(*in.Username)
	}
	if in.Role != nil {
		r := strings.TrimSpace(*in.Role)
		if r != models.RoleAdmin && r != models.RoleUser {
			return nil, ErrInvalidRole
		}
		if u.Role == models.RoleAdmin && r != models.RoleAdmin {
			n, err := s.repo.CountByRole(ctx, models.RoleAdmin)
			if err != nil {
				return nil, err
			}
			if n <= 1 {
				return nil, ErrLastAdmin
			}
		}
		u.Role = r
	}
	if in.Password != nil && *in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = string(hash)
	}
	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID, actorUsername string) error {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}
	if u.Username == actorUsername {
		return ErrCannotDeleteSelf
	}
	if u.Role == models.RoleAdmin {
		n, err := s.repo.CountByRole(ctx, models.RoleAdmin)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	return s.repo.Delete(ctx, id)
}
