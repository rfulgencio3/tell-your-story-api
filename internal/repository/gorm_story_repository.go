package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormStoryRepository stores stories in PostgreSQL using GORM.
type GormStoryRepository struct {
	db *gorm.DB
}

// NewGormStoryRepository creates a GORM-backed story repository.
func NewGormStoryRepository(db *gorm.DB) *GormStoryRepository {
	return &GormStoryRepository{db: db}
}

// Create stores a story.
func (r *GormStoryRepository) Create(ctx context.Context, story domain.Story) error {
	if err := r.db.WithContext(ctx).Create(&story).Error; err != nil {
		return fmt.Errorf("insert story: %w", err)
	}

	return nil
}

// GetByID fetches a story by id.
func (r *GormStoryRepository) GetByID(ctx context.Context, id string) (domain.Story, error) {
	var story domain.Story
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&story).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Story{}, domain.ErrStoryNotFound
		}
		return domain.Story{}, fmt.Errorf("select story by id: %w", err)
	}

	return story, nil
}

// GetByUserAndRound returns the user's story for the round if present.
func (r *GormStoryRepository) GetByUserAndRound(ctx context.Context, userID, roundID string) (domain.Story, error) {
	var story domain.Story
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND round_id = ?", userID, roundID).
		First(&story).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Story{}, domain.ErrStoryNotFound
		}
		return domain.Story{}, fmt.Errorf("select story by user and round: %w", err)
	}

	return story, nil
}

// ListByRoundID returns all stories in a round.
func (r *GormStoryRepository) ListByRoundID(ctx context.Context, roundID string) ([]domain.Story, error) {
	var stories []domain.Story
	if err := r.db.WithContext(ctx).
		Where("round_id = ?", roundID).
		Order("created_at asc").
		Find(&stories).Error; err != nil {
		return nil, fmt.Errorf("list stories by round: %w", err)
	}

	return stories, nil
}

// Update overwrites a story.
func (r *GormStoryRepository) Update(ctx context.Context, story domain.Story) error {
	result := r.db.WithContext(ctx).Model(&domain.Story{}).Where("id = ?", story.ID).Select("*").Updates(&story)
	if result.Error != nil {
		return fmt.Errorf("update story: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrStoryNotFound
	}

	return nil
}

var _ StoryRepository = (*GormStoryRepository)(nil)
