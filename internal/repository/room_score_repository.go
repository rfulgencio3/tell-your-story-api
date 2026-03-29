package repository

import (
	"context"
	"sync"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/pkg/utils"
)

// RoomScoreRepository defines persistence operations for accumulated room scores.
type RoomScoreRepository interface {
	Increment(ctx context.Context, roomID, userID string, delta int, updatedAt time.Time) (domain.RoomScore, error)
	ListByRoomID(ctx context.Context, roomID string) ([]domain.RoomScore, error)
}

// InMemoryRoomScoreRepository stores room scores in memory.
type InMemoryRoomScoreRepository struct {
	mu            sync.RWMutex
	byID          map[string]domain.RoomScore
	byRoomUser    map[string]string
	roomToScoreID map[string][]string
}

// NewInMemoryRoomScoreRepository creates an in-memory room score repository.
func NewInMemoryRoomScoreRepository() *InMemoryRoomScoreRepository {
	return &InMemoryRoomScoreRepository{
		byID:          make(map[string]domain.RoomScore),
		byRoomUser:    make(map[string]string),
		roomToScoreID: make(map[string][]string),
	}
}

// Increment creates or updates a participant score within a room.
func (r *InMemoryRoomScoreRepository) Increment(_ context.Context, roomID, userID string, delta int, updatedAt time.Time) (domain.RoomScore, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := roomScoreKey(roomID, userID)
	if scoreID, exists := r.byRoomUser[key]; exists {
		score := r.byID[scoreID]
		score.Score += delta
		score.UpdatedAt = updatedAt
		r.byID[scoreID] = score
		return score, nil
	}

	scoreID, err := utils.GenerateID()
	if err != nil {
		return domain.RoomScore{}, err
	}

	score := domain.RoomScore{
		ID:        scoreID,
		RoomID:    roomID,
		UserID:    userID,
		Score:     delta,
		UpdatedAt: updatedAt,
	}
	r.byID[scoreID] = score
	r.byRoomUser[key] = scoreID
	r.roomToScoreID[roomID] = append(r.roomToScoreID[roomID], scoreID)

	return score, nil
}

// ListByRoomID returns all persisted scores for a room.
func (r *InMemoryRoomScoreRepository) ListByRoomID(_ context.Context, roomID string) ([]domain.RoomScore, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scoreIDs := r.roomToScoreID[roomID]
	scores := make([]domain.RoomScore, 0, len(scoreIDs))
	for _, scoreID := range scoreIDs {
		score, exists := r.byID[scoreID]
		if exists {
			scores = append(scores, score)
		}
	}

	return scores, nil
}

func roomScoreKey(roomID, userID string) string {
	return roomID + "::" + userID
}

var _ RoomScoreRepository = (*InMemoryRoomScoreRepository)(nil)
