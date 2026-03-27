package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormUserRepository stores users in PostgreSQL using GORM.
type GormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository creates a GORM-backed user repository.
func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

// Create stores a user.
func (r *GormUserRepository) Create(ctx context.Context, user domain.User) error {
	if err := r.db.WithContext(ctx).Create(&user).Error; err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// GetByID fetches a user by id.
func (r *GormUserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("select user by id: %w", err)
	}

	return user, nil
}

// ListByRoomID returns all users currently in a room.
func (r *GormUserRepository) ListByRoomID(ctx context.Context, roomID string) ([]domain.User, error) {
	var users []domain.User
	if err := r.db.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("created_at asc").
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list users by room: %w", err)
	}

	return users, nil
}

// Delete removes a user from the repository.
func (r *GormUserRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&domain.User{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

var _ UserRepository = (*GormUserRepository)(nil)
