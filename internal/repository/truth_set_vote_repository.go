package repository

import (
	"context"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// TruthSetVoteRepository defines persistence operations for three-lies truth set votes.
type TruthSetVoteRepository interface {
	Upsert(ctx context.Context, vote domain.TruthSetVote) error
	GetByTruthSetAndUser(ctx context.Context, truthSetID, userID string) (domain.TruthSetVote, error)
	ListByTruthSetID(ctx context.Context, truthSetID string) ([]domain.TruthSetVote, error)
}

// InMemoryTruthSetVoteRepository stores truth set votes in memory.
type InMemoryTruthSetVoteRepository struct {
	mu               sync.RWMutex
	byID             map[string]domain.TruthSetVote
	truthSetToVoteID map[string][]string
	byTruthSetUser   map[string]string
}

// NewInMemoryTruthSetVoteRepository creates an in-memory truth set vote repository.
func NewInMemoryTruthSetVoteRepository() *InMemoryTruthSetVoteRepository {
	return &InMemoryTruthSetVoteRepository{
		byID:             make(map[string]domain.TruthSetVote),
		truthSetToVoteID: make(map[string][]string),
		byTruthSetUser:   make(map[string]string),
	}
}

// Upsert stores a truth set vote or overwrites the user's existing vote for the same truth set.
func (r *InMemoryTruthSetVoteRepository) Upsert(_ context.Context, vote domain.TruthSetVote) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := truthSetVoteKey(vote.TruthSetID, vote.UserID)
	if existingID, exists := r.byTruthSetUser[key]; exists {
		vote.ID = existingID
		r.byID[existingID] = vote
		return nil
	}

	r.byID[vote.ID] = vote
	r.truthSetToVoteID[vote.TruthSetID] = append(r.truthSetToVoteID[vote.TruthSetID], vote.ID)
	r.byTruthSetUser[key] = vote.ID
	return nil
}

// GetByTruthSetAndUser returns the user's vote for a truth set if present.
func (r *InMemoryTruthSetVoteRepository) GetByTruthSetAndUser(_ context.Context, truthSetID, userID string) (domain.TruthSetVote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	voteID, exists := r.byTruthSetUser[truthSetVoteKey(truthSetID, userID)]
	if !exists {
		return domain.TruthSetVote{}, domain.ErrTruthSetVoteNotFound
	}

	vote, exists := r.byID[voteID]
	if !exists {
		return domain.TruthSetVote{}, domain.ErrTruthSetVoteNotFound
	}

	return vote, nil
}

// ListByTruthSetID returns all votes cast for a truth set.
func (r *InMemoryTruthSetVoteRepository) ListByTruthSetID(_ context.Context, truthSetID string) ([]domain.TruthSetVote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.truthSetToVoteID[truthSetID]
	votes := make([]domain.TruthSetVote, 0, len(ids))
	for _, id := range ids {
		vote, exists := r.byID[id]
		if exists {
			votes = append(votes, vote)
		}
	}

	return votes, nil
}

func truthSetVoteKey(truthSetID, userID string) string {
	return truthSetID + "::" + userID
}

var _ TruthSetVoteRepository = (*InMemoryTruthSetVoteRepository)(nil)
