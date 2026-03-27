package repository

import (
	"context"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// StoryRepository defines persistence operations for stories.
type StoryRepository interface {
	Create(ctx context.Context, story domain.Story) error
	GetByID(ctx context.Context, id string) (domain.Story, error)
	GetByUserAndRound(ctx context.Context, userID, roundID string) (domain.Story, error)
	ListByRoundID(ctx context.Context, roundID string) ([]domain.Story, error)
	Update(ctx context.Context, story domain.Story) error
}

// InMemoryStoryRepository stores stories in memory.
type InMemoryStoryRepository struct {
	mu           sync.RWMutex
	byID         map[string]domain.Story
	roundToStory map[string][]string
}

// NewInMemoryStoryRepository creates a story repository backed by memory.
func NewInMemoryStoryRepository() *InMemoryStoryRepository {
	return &InMemoryStoryRepository{
		byID:         make(map[string]domain.Story),
		roundToStory: make(map[string][]string),
	}
}

// Create stores a story.
func (r *InMemoryStoryRepository) Create(_ context.Context, story domain.Story) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[story.ID] = story
	r.roundToStory[story.RoundID] = append(r.roundToStory[story.RoundID], story.ID)

	return nil
}

// GetByID fetches a story by id.
func (r *InMemoryStoryRepository) GetByID(_ context.Context, id string) (domain.Story, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	story, exists := r.byID[id]
	if !exists {
		return domain.Story{}, domain.ErrStoryNotFound
	}

	return story, nil
}

// GetByUserAndRound returns the user's story for the round if present.
func (r *InMemoryStoryRepository) GetByUserAndRound(_ context.Context, userID, roundID string) (domain.Story, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, storyID := range r.roundToStory[roundID] {
		story, exists := r.byID[storyID]
		if exists && story.UserID == userID {
			return story, nil
		}
	}

	return domain.Story{}, domain.ErrStoryNotFound
}

// ListByRoundID returns all stories in a round.
func (r *InMemoryStoryRepository) ListByRoundID(_ context.Context, roundID string) ([]domain.Story, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.roundToStory[roundID]
	stories := make([]domain.Story, 0, len(ids))
	for _, id := range ids {
		story, exists := r.byID[id]
		if exists {
			stories = append(stories, story)
		}
	}

	return stories, nil
}

// Update overwrites a story.
func (r *InMemoryStoryRepository) Update(_ context.Context, story domain.Story) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[story.ID]; !exists {
		return domain.ErrStoryNotFound
	}

	r.byID[story.ID] = story

	return nil
}

var _ StoryRepository = (*InMemoryStoryRepository)(nil)
