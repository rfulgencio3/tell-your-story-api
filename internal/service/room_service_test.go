package service

import (
	"context"
	"testing"
	"time"

	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

func TestCreateRoom(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		HostAvatar:   "fox",
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	if state.Room.Status != domain.RoomStatusWaiting {
		t.Fatalf("room status = %q, want %q", state.Room.Status, domain.RoomStatusWaiting)
	}

	if len(state.Users) != 1 {
		t.Fatalf("users count = %d, want 1", len(state.Users))
	}

	if !state.Users[0].IsHost {
		t.Fatal("created user should be the host")
	}
}

func TestJoinRoomRejectsWhenFull(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 2,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		MaxRounds:    2,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	if _, err := svc.JoinRoom(context.Background(), state.Room.Code, JoinRoomInput{
		Nickname: "Ana",
	}); err != nil {
		t.Fatalf("JoinRoom() unexpected error = %v", err)
	}

	if _, err := svc.JoinRoom(context.Background(), state.Room.Code, JoinRoomInput{
		Nickname: "Bob",
	}); err != domain.ErrRoomFull {
		t.Fatalf("JoinRoom() error = %v, want %v", err, domain.ErrRoomFull)
	}
}

func TestStartGameCreatesFirstRound(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := svc.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID: state.Room.HostID,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	if started.Room.Status != domain.RoomStatusActive {
		t.Fatalf("room status = %q, want %q", started.Room.Status, domain.RoomStatusActive)
	}

	if started.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}

	if started.CurrentRound.RoundNumber != 1 {
		t.Fatalf("round number = %d, want 1", started.CurrentRound.RoundNumber)
	}
}

func TestNextRoundFinishesRoomOnLastRound(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		MaxRounds:    1,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := svc.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID: state.Room.HostID,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	finished, err := svc.NextRound(context.Background(), state.Room.Code, RoomActionInput{
		UserID: state.Room.HostID,
	})
	if err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}

	if finished.Room.Status != domain.RoomStatusFinished {
		t.Fatalf("room status = %q, want %q", finished.Room.Status, domain.RoomStatusFinished)
	}

	if finished.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}

	if finished.CurrentRound.ID != started.CurrentRound.ID {
		t.Fatal("final state should still point to the completed last round")
	}
}

func newTestRoomService(cfg config.GameConfig) *RoomService {
	return NewRoomService(
		cfg,
		repository.NewInMemoryRoomRepository(),
		repository.NewInMemoryUserRepository(),
		repository.NewInMemoryRoundRepository(),
	)
}
