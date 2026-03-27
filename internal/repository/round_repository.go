package repository

import (
	"context"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// RoundRepository defines persistence operations for rounds.
type RoundRepository interface {
	Create(ctx context.Context, round domain.Round) error
	GetByID(ctx context.Context, id string) (domain.Round, error)
	GetCurrentByRoomID(ctx context.Context, roomID string) (domain.Round, error)
	Update(ctx context.Context, round domain.Round) error
}

// InMemoryRoundRepository stores rounds in memory.
type InMemoryRoundRepository struct {
	mu          sync.RWMutex
	byID        map[string]domain.Round
	roomToRound map[string][]string
}

// NewInMemoryRoundRepository creates a round repository backed by memory.
func NewInMemoryRoundRepository() *InMemoryRoundRepository {
	return &InMemoryRoundRepository{
		byID:        make(map[string]domain.Round),
		roomToRound: make(map[string][]string),
	}
}

// Create stores a round.
func (r *InMemoryRoundRepository) Create(_ context.Context, round domain.Round) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[round.ID] = round
	r.roomToRound[round.RoomID] = append(r.roomToRound[round.RoomID], round.ID)

	return nil
}

// GetByID fetches a round by id.
func (r *InMemoryRoundRepository) GetByID(_ context.Context, id string) (domain.Round, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	round, exists := r.byID[id]
	if !exists {
		return domain.Round{}, domain.ErrRoundNotFound
	}

	return round, nil
}

// GetCurrentByRoomID returns the latest round for a room.
func (r *InMemoryRoundRepository) GetCurrentByRoomID(_ context.Context, roomID string) (domain.Round, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.roomToRound[roomID]
	if len(ids) == 0 {
		return domain.Round{}, domain.ErrRoundNotFound
	}

	lastID := ids[len(ids)-1]
	round, exists := r.byID[lastID]
	if !exists {
		return domain.Round{}, domain.ErrRoundNotFound
	}

	return round, nil
}

// Update overwrites a round.
func (r *InMemoryRoundRepository) Update(_ context.Context, round domain.Round) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[round.ID]; !exists {
		return domain.ErrRoundNotFound
	}

	r.byID[round.ID] = round

	return nil
}

var _ RoundRepository = (*InMemoryRoundRepository)(nil)
