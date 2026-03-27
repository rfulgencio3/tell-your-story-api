package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormRoomRepository stores rooms in PostgreSQL using GORM.
type GormRoomRepository struct {
	db *gorm.DB
}

// NewGormRoomRepository creates a GORM-backed room repository.
func NewGormRoomRepository(db *gorm.DB) *GormRoomRepository {
	return &GormRoomRepository{db: db}
}

// Create stores a room.
func (r *GormRoomRepository) Create(ctx context.Context, room domain.Room) error {
	if err := r.db.WithContext(ctx).Create(&room).Error; err != nil {
		return fmt.Errorf("insert room: %w", err)
	}

	return nil
}

// GetByCode fetches a room by its human-friendly code.
func (r *GormRoomRepository) GetByCode(ctx context.Context, code string) (domain.Room, error) {
	var room domain.Room
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&room).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Room{}, domain.ErrRoomNotFound
		}
		return domain.Room{}, fmt.Errorf("select room by code: %w", err)
	}

	return room, nil
}

// Update overwrites a room.
func (r *GormRoomRepository) Update(ctx context.Context, room domain.Room) error {
	result := r.db.WithContext(ctx).Model(&domain.Room{}).Where("id = ?", room.ID).Select("*").Updates(&room)
	if result.Error != nil {
		return fmt.Errorf("update room: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrRoomNotFound
	}

	return nil
}

var _ RoomRepository = (*GormRoomRepository)(nil)
