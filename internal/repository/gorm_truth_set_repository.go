package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormTruthSetRepository stores truth sets in PostgreSQL using GORM.
type GormTruthSetRepository struct {
	db *gorm.DB
}

// NewGormTruthSetRepository creates a GORM-backed truth set repository.
func NewGormTruthSetRepository(db *gorm.DB) *GormTruthSetRepository {
	return &GormTruthSetRepository{db: db}
}

// Create stores a truth set with its normalized statements.
func (r *GormTruthSetRepository) Create(ctx context.Context, truthSet domain.TruthSet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		truthSetModel := truthSet
		truthSetModel.Statements = nil
		if err := tx.Create(&truthSetModel).Error; err != nil {
			return fmt.Errorf("insert truth set: %w", err)
		}

		if len(truthSet.Statements) > 0 {
			if err := tx.Create(&truthSet.Statements).Error; err != nil {
				return fmt.Errorf("insert truth set statements: %w", err)
			}
		}

		return nil
	})
}

// GetByID fetches a truth set by id.
func (r *GormTruthSetRepository) GetByID(ctx context.Context, id string) (domain.TruthSet, error) {
	var truthSet domain.TruthSet
	if err := r.truthSetQuery(ctx).Where("id = ?", id).First(&truthSet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.TruthSet{}, domain.ErrTruthSetNotFound
		}
		return domain.TruthSet{}, fmt.Errorf("select truth set by id: %w", err)
	}

	return truthSet, nil
}

// GetByAuthorAndRound fetches the truth set authored by a user in a round.
func (r *GormTruthSetRepository) GetByAuthorAndRound(ctx context.Context, authorUserID, roundID string) (domain.TruthSet, error) {
	var truthSet domain.TruthSet
	if err := r.truthSetQuery(ctx).
		Where("author_user_id = ? AND round_id = ?", authorUserID, roundID).
		First(&truthSet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.TruthSet{}, domain.ErrTruthSetNotFound
		}
		return domain.TruthSet{}, fmt.Errorf("select truth set by author and round: %w", err)
	}

	return truthSet, nil
}

// ListByRoundID returns all truth sets in a round.
func (r *GormTruthSetRepository) ListByRoundID(ctx context.Context, roundID string) ([]domain.TruthSet, error) {
	var truthSets []domain.TruthSet
	if err := r.truthSetQuery(ctx).
		Where("round_id = ?", roundID).
		Order("created_at asc").
		Find(&truthSets).Error; err != nil {
		return nil, fmt.Errorf("list truth sets by round: %w", err)
	}

	return truthSets, nil
}

// Update overwrites a truth set and replaces its statements.
func (r *GormTruthSetRepository) Update(ctx context.Context, truthSet domain.TruthSet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		truthSetModel := truthSet
		truthSetModel.Statements = nil
		result := tx.Model(&domain.TruthSet{}).Where("id = ?", truthSet.ID).Select("*").Updates(&truthSetModel)
		if result.Error != nil {
			return fmt.Errorf("update truth set: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrTruthSetNotFound
		}

		if err := tx.Where("truth_set_id = ?", truthSet.ID).Delete(&domain.TruthSetStatement{}).Error; err != nil {
			return fmt.Errorf("delete truth set statements: %w", err)
		}

		if len(truthSet.Statements) > 0 {
			if err := tx.Create(&truthSet.Statements).Error; err != nil {
				return fmt.Errorf("replace truth set statements: %w", err)
			}
		}

		return nil
	})
}

func (r *GormTruthSetRepository) truthSetQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Preload("Statements", func(db *gorm.DB) *gorm.DB {
		return db.Order("statement_index asc")
	})
}

var _ TruthSetRepository = (*GormTruthSetRepository)(nil)
