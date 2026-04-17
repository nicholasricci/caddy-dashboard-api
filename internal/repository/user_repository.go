package repository

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) List(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).Order("username asc").Limit(100).Find(&users).Error
	return users, err
}

func (r *UserRepository) ListPaginated(ctx context.Context, limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64
	q := r.db.WithContext(ctx).Model(&models.User{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("username asc").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "username = ?", username).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id).Error
}

func (r *UserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Where("role = ?", role).Count(&n).Error
	return n, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func IsDuplicate(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062
}

func (r *UserRepository) DB() *gorm.DB {
	return r.db
}
