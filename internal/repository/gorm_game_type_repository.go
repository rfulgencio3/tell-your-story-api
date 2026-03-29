package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormGameTypeRepository stores game types in PostgreSQL using GORM.
type GormGameTypeRepository struct {
	db *gorm.DB
}

// NewGormGameTypeRepository creates a GORM-backed game type repository.
func NewGormGameTypeRepository(db *gorm.DB) *GormGameTypeRepository {
	return &GormGameTypeRepository{db: db}
}

// EnsureDefaults inserts the fixed game type catalog entries when missing.
func (r *GormGameTypeRepository) EnsureDefaults(ctx context.Context) error {
	now := time.Now().UTC()
	for _, seed := range defaultGameTypeSeeds {
		var stored domain.GameType
		err := r.db.WithContext(ctx).Where("slug = ?", seed.Slug).First(&stored).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			gameType := domain.GameType{
				ID:          seed.ID,
				Slug:        seed.Slug,
				Name:        seed.Name,
				Description: seed.Description,
				IsActive:    true,
				CreatedAt:   now,
			}
			if createErr := r.db.WithContext(ctx).Create(&gameType).Error; createErr != nil {
				return fmt.Errorf("insert default game type %q: %w", seed.Slug, createErr)
			}
		case err != nil:
			return fmt.Errorf("select game type by slug %q: %w", seed.Slug, err)
		default:
			updates := map[string]any{
				"name":        seed.Name,
				"description": seed.Description,
				"is_active":   true,
			}
			if updateErr := r.db.WithContext(ctx).Model(&domain.GameType{}).Where("id = ?", stored.ID).Updates(updates).Error; updateErr != nil {
				return fmt.Errorf("update default game type %q: %w", seed.Slug, updateErr)
			}
		}
	}

	return nil
}

// GetBySlug fetches an active game type by slug.
func (r *GormGameTypeRepository) GetBySlug(ctx context.Context, slug string) (domain.GameType, error) {
	var gameType domain.GameType
	if err := r.db.WithContext(ctx).Where("slug = ? AND is_active = ?", slug, true).First(&gameType).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GameType{}, domain.ErrGameTypeNotFound
		}
		return domain.GameType{}, fmt.Errorf("select game type by slug: %w", err)
	}

	return gameType, nil
}

var _ GameTypeRepository = (*GormGameTypeRepository)(nil)
