package handlers

import "context"

// RealtimeNotifier publishes room-scoped websocket events.
type RealtimeNotifier interface {
	BroadcastRoomState(ctx context.Context, roomCode string) error
	BroadcastStoryProgress(ctx context.Context, roundID string) error
	BroadcastVoteProgress(ctx context.Context, roundID string) error
	BroadcastTopStory(ctx context.Context, roundID string, payload any) error
}
