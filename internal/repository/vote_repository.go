package repository

import (
	"context"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// VoteRepository defines persistence operations for votes.
type VoteRepository interface {
	Create(ctx context.Context, vote domain.Vote) error
	GetByUserAndRound(ctx context.Context, userID, roundID string) (domain.Vote, error)
	ListByRoundID(ctx context.Context, roundID string) ([]domain.Vote, error)
}

// InMemoryVoteRepository stores votes in memory.
type InMemoryVoteRepository struct {
	mu          sync.RWMutex
	byID        map[string]domain.Vote
	roundToVote map[string][]string
}

// NewInMemoryVoteRepository creates a vote repository backed by memory.
func NewInMemoryVoteRepository() *InMemoryVoteRepository {
	return &InMemoryVoteRepository{
		byID:        make(map[string]domain.Vote),
		roundToVote: make(map[string][]string),
	}
}

// Create stores a vote.
func (r *InMemoryVoteRepository) Create(_ context.Context, vote domain.Vote) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[vote.ID] = vote
	r.roundToVote[vote.RoundID] = append(r.roundToVote[vote.RoundID], vote.ID)

	return nil
}

// GetByUserAndRound returns the user's vote for the round if present.
func (r *InMemoryVoteRepository) GetByUserAndRound(_ context.Context, userID, roundID string) (domain.Vote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, voteID := range r.roundToVote[roundID] {
		vote, exists := r.byID[voteID]
		if exists && vote.UserID == userID {
			return vote, nil
		}
	}

	return domain.Vote{}, domain.ErrVoteNotFound
}

// ListByRoundID returns all votes in a round.
func (r *InMemoryVoteRepository) ListByRoundID(_ context.Context, roundID string) ([]domain.Vote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.roundToVote[roundID]
	votes := make([]domain.Vote, 0, len(ids))
	for _, id := range ids {
		vote, exists := r.byID[id]
		if exists {
			votes = append(votes, vote)
		}
	}

	return votes, nil
}

var _ VoteRepository = (*InMemoryVoteRepository)(nil)
