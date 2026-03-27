package repository

import (
	"context"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	GetByID(ctx context.Context, id string) (domain.User, error)
	ListByRoomID(ctx context.Context, roomID string) ([]domain.User, error)
	Delete(ctx context.Context, id string) error
}

// InMemoryUserRepository stores users in memory.
type InMemoryUserRepository struct {
	mu         sync.RWMutex
	byID       map[string]domain.User
	roomToUser map[string][]string
}

// NewInMemoryUserRepository creates a user repository backed by memory.
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		byID:       make(map[string]domain.User),
		roomToUser: make(map[string][]string),
	}
}

// Create stores a user.
func (r *InMemoryUserRepository) Create(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[user.ID] = user
	r.roomToUser[user.RoomID] = append(r.roomToUser[user.RoomID], user.ID)

	return nil
}

// GetByID fetches a user by id.
func (r *InMemoryUserRepository) GetByID(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.byID[id]
	if !exists {
		return domain.User{}, domain.ErrUserNotFound
	}

	return user, nil
}

// ListByRoomID returns all users currently in a room.
func (r *InMemoryUserRepository) ListByRoomID(_ context.Context, roomID string) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.roomToUser[roomID]
	users := make([]domain.User, 0, len(ids))
	for _, id := range ids {
		user, exists := r.byID[id]
		if exists {
			users = append(users, user)
		}
	}

	return users, nil
}

// Delete removes a user from the repository.
func (r *InMemoryUserRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.byID[id]
	if !exists {
		return domain.ErrUserNotFound
	}

	delete(r.byID, id)

	ids := r.roomToUser[user.RoomID]
	filtered := make([]string, 0, len(ids))
	for _, userID := range ids {
		if userID != id {
			filtered = append(filtered, userID)
		}
	}

	r.roomToUser[user.RoomID] = filtered

	return nil
}

var _ UserRepository = (*InMemoryUserRepository)(nil)
