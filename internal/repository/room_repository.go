package repository

import (
	"context"
	"sync"

	"github.com/tell-your-story/backend/internal/domain"
)

// RoomRepository defines persistence operations for rooms.
type RoomRepository interface {
	Create(ctx context.Context, room domain.Room) error
	GetByID(ctx context.Context, id string) (domain.Room, error)
	GetByCode(ctx context.Context, code string) (domain.Room, error)
	Update(ctx context.Context, room domain.Room) error
}

// InMemoryRoomRepository stores rooms in memory.
type InMemoryRoomRepository struct {
	mu     sync.RWMutex
	byID   map[string]domain.Room
	byCode map[string]string
}

// NewInMemoryRoomRepository creates a room repository backed by memory.
func NewInMemoryRoomRepository() *InMemoryRoomRepository {
	return &InMemoryRoomRepository{
		byID:   make(map[string]domain.Room),
		byCode: make(map[string]string),
	}
}

// Create stores a room.
func (r *InMemoryRoomRepository) Create(_ context.Context, room domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[room.ID] = room
	r.byCode[room.Code] = room.ID

	return nil
}

// GetByCode fetches a room by its human-friendly code.
func (r *InMemoryRoomRepository) GetByCode(_ context.Context, code string) (domain.Room, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roomID, exists := r.byCode[code]
	if !exists {
		return domain.Room{}, domain.ErrRoomNotFound
	}

	room, exists := r.byID[roomID]
	if !exists {
		return domain.Room{}, domain.ErrRoomNotFound
	}

	return room, nil
}

// GetByID fetches a room by its id.
func (r *InMemoryRoomRepository) GetByID(_ context.Context, id string) (domain.Room, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	room, exists := r.byID[id]
	if !exists {
		return domain.Room{}, domain.ErrRoomNotFound
	}

	return room, nil
}

// Update overwrites a room.
func (r *InMemoryRoomRepository) Update(_ context.Context, room domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[room.ID]; !exists {
		return domain.ErrRoomNotFound
	}

	r.byID[room.ID] = room
	r.byCode[room.Code] = room.ID

	return nil
}

var _ RoomRepository = (*InMemoryRoomRepository)(nil)
