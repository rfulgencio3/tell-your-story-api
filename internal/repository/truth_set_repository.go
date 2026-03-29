package repository

import (
	"context"
	"sort"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// TruthSetRepository defines persistence operations for three-lies truth sets.
type TruthSetRepository interface {
	Create(ctx context.Context, truthSet domain.TruthSet) error
	GetByID(ctx context.Context, id string) (domain.TruthSet, error)
	GetByAuthorAndRound(ctx context.Context, authorUserID, roundID string) (domain.TruthSet, error)
	ListByRoundID(ctx context.Context, roundID string) ([]domain.TruthSet, error)
	Update(ctx context.Context, truthSet domain.TruthSet) error
}

// InMemoryTruthSetRepository stores truth sets in memory.
type InMemoryTruthSetRepository struct {
	mu            sync.RWMutex
	byID          map[string]domain.TruthSet
	roundToSetIDs map[string][]string
	byRoundAuthor map[string]string
}

// NewInMemoryTruthSetRepository creates a truth set repository backed by memory.
func NewInMemoryTruthSetRepository() *InMemoryTruthSetRepository {
	return &InMemoryTruthSetRepository{
		byID:          make(map[string]domain.TruthSet),
		roundToSetIDs: make(map[string][]string),
		byRoundAuthor: make(map[string]string),
	}
}

// Create stores a truth set with its statements.
func (r *InMemoryTruthSetRepository) Create(_ context.Context, truthSet domain.TruthSet) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[truthSet.ID] = cloneTruthSet(truthSet)
	r.roundToSetIDs[truthSet.RoundID] = append(r.roundToSetIDs[truthSet.RoundID], truthSet.ID)
	r.byRoundAuthor[roundAuthorKey(truthSet.AuthorUserID, truthSet.RoundID)] = truthSet.ID

	return nil
}

// GetByID fetches a truth set by id.
func (r *InMemoryTruthSetRepository) GetByID(_ context.Context, id string) (domain.TruthSet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	truthSet, exists := r.byID[id]
	if !exists {
		return domain.TruthSet{}, domain.ErrTruthSetNotFound
	}

	return cloneTruthSet(truthSet), nil
}

// GetByAuthorAndRound fetches the truth set authored by a user in a round.
func (r *InMemoryTruthSetRepository) GetByAuthorAndRound(_ context.Context, authorUserID, roundID string) (domain.TruthSet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	truthSetID, exists := r.byRoundAuthor[roundAuthorKey(authorUserID, roundID)]
	if !exists {
		return domain.TruthSet{}, domain.ErrTruthSetNotFound
	}

	truthSet, exists := r.byID[truthSetID]
	if !exists {
		return domain.TruthSet{}, domain.ErrTruthSetNotFound
	}

	return cloneTruthSet(truthSet), nil
}

// ListByRoundID returns all truth sets in a round.
func (r *InMemoryTruthSetRepository) ListByRoundID(_ context.Context, roundID string) ([]domain.TruthSet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.roundToSetIDs[roundID]
	truthSets := make([]domain.TruthSet, 0, len(ids))
	for _, id := range ids {
		truthSet, exists := r.byID[id]
		if exists {
			truthSets = append(truthSets, cloneTruthSet(truthSet))
		}
	}

	sort.Slice(truthSets, func(i, j int) bool {
		return truthSets[i].CreatedAt.Before(truthSets[j].CreatedAt)
	})

	return truthSets, nil
}

// Update overwrites a truth set and its statements.
func (r *InMemoryTruthSetRepository) Update(_ context.Context, truthSet domain.TruthSet) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[truthSet.ID]; !exists {
		return domain.ErrTruthSetNotFound
	}

	r.byID[truthSet.ID] = cloneTruthSet(truthSet)
	return nil
}

func cloneTruthSet(truthSet domain.TruthSet) domain.TruthSet {
	cloned := truthSet
	cloned.Statements = append([]domain.TruthSetStatement(nil), truthSet.Statements...)
	sort.Slice(cloned.Statements, func(i, j int) bool {
		return cloned.Statements[i].StatementIndex < cloned.Statements[j].StatementIndex
	})
	return cloned
}

func roundAuthorKey(authorUserID, roundID string) string {
	return roundID + "::" + authorUserID
}

var _ TruthSetRepository = (*InMemoryTruthSetRepository)(nil)
