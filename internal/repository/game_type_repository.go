package repository

import (
	"context"
	"sync"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
)

type gameTypeSeed struct {
	ID          string
	Slug        string
	Name        string
	Description string
}

var defaultGameTypeSeeds = []gameTypeSeed{
	{
		ID:          domain.GameTypeIDTellYourStory,
		Slug:        domain.GameTypeTellYourStory,
		Name:        "Tell Your Story",
		Description: "Classic story writing and voting mode",
	},
	{
		ID:          domain.GameTypeIDThreeLiesOneTruth,
		Slug:        domain.GameTypeThreeLiesOneTruth,
		Name:        "Three Lies, One Truth",
		Description: "Players write four statements and others guess the truth",
	},
}

// GameTypeRepository defines persistence operations for game mode catalog entries.
type GameTypeRepository interface {
	EnsureDefaults(ctx context.Context) error
	GetBySlug(ctx context.Context, slug string) (domain.GameType, error)
}

// InMemoryGameTypeRepository stores game types in memory.
type InMemoryGameTypeRepository struct {
	mu     sync.RWMutex
	bySlug map[string]domain.GameType
}

// NewInMemoryGameTypeRepository creates an in-memory repository seeded with default game types.
func NewInMemoryGameTypeRepository() *InMemoryGameTypeRepository {
	repo := &InMemoryGameTypeRepository{
		bySlug: make(map[string]domain.GameType, len(defaultGameTypeSeeds)),
	}
	_ = repo.EnsureDefaults(context.Background())
	return repo
}

// EnsureDefaults inserts the fixed game types when they are missing.
func (r *InMemoryGameTypeRepository) EnsureDefaults(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for _, seed := range defaultGameTypeSeeds {
		if _, exists := r.bySlug[seed.Slug]; exists {
			continue
		}

		r.bySlug[seed.Slug] = domain.GameType{
			ID:          seed.ID,
			Slug:        seed.Slug,
			Name:        seed.Name,
			Description: seed.Description,
			IsActive:    true,
			CreatedAt:   now,
		}
	}

	return nil
}

// GetBySlug fetches an active game type by slug.
func (r *InMemoryGameTypeRepository) GetBySlug(_ context.Context, slug string) (domain.GameType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	gameType, exists := r.bySlug[slug]
	if !exists || !gameType.IsActive {
		return domain.GameType{}, domain.ErrGameTypeNotFound
	}

	return gameType, nil
}

var _ GameTypeRepository = (*InMemoryGameTypeRepository)(nil)
