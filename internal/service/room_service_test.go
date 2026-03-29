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

	if state.Users[0].SessionToken == "" {
		t.Fatal("created host should receive a session token")
	}

	if state.Room.GameType != domain.GameTypeTellYourStory {
		t.Fatalf("room game_type = %q, want %q", state.Room.GameType, domain.GameTypeTellYourStory)
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

func TestCreateRoomRejectsUnsupportedGameType(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	if _, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		GameType:     "unknown-mode",
		MaxRounds:    3,
		TimePerRound: 120,
	}); err != domain.ErrGameTypeNotFound {
		t.Fatalf("CreateRoom() error = %v, want %v", err, domain.ErrGameTypeNotFound)
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
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
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

func TestStartGameCreatesCountdownRoundForThreeLiesOneTruth(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		GameType:     domain.GameTypeThreeLiesOneTruth,
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := svc.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	if started.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}

	if started.CurrentRound.Status != domain.RoundStatusCountdown {
		t.Fatalf("round status = %q, want %q", started.CurrentRound.Status, domain.RoundStatusCountdown)
	}
}

func TestGetRoomStateAdvancesThreeLiesCountdownToWriting(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		GameType:     domain.GameTypeThreeLiesOneTruth,
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := svc.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	expiredAt := time.Now().UTC().Add(-time.Second)
	started.CurrentRound.PhaseEndsAt = &expiredAt
	if err := svc.roundRepo.Update(context.Background(), *started.CurrentRound); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	synced, err := svc.GetRoomState(context.Background(), state.Room.Code)
	if err != nil {
		t.Fatalf("GetRoomState() error = %v", err)
	}

	if synced.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}

	if synced.CurrentRound.Status != domain.RoundStatusWriting {
		t.Fatalf("round status = %q, want %q", synced.CurrentRound.Status, domain.RoundStatusWriting)
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
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	voting, err := svc.NextRound(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}

	if voting.CurrentRound == nil || voting.CurrentRound.Status != domain.RoundStatusVoting {
		t.Fatalf("round status = %v, want %v", voting.CurrentRound.Status, domain.RoundStatusVoting)
	}

	revealed, err := svc.NextRound(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}

	if revealed.CurrentRound == nil || revealed.CurrentRound.Status != domain.RoundStatusRevealed {
		t.Fatalf("round status = %v, want %v", revealed.CurrentRound.Status, domain.RoundStatusRevealed)
	}

	finished, err := svc.NextRound(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}

	if finished.Room.Status != domain.RoomStatusFinished {
		t.Fatalf("room status = %q, want %q", finished.Room.Status, domain.RoomStatusFinished)
	}

	if finished.CurrentRound == nil || finished.CurrentRound.ID != started.CurrentRound.ID {
		t.Fatal("final state should still point to the completed last round")
	}
}

func TestNextRoundStartsNewRoundAfterReveal(t *testing.T) {
	t.Parallel()

	svc := newTestRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	})

	state, err := svc.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		MaxRounds:    2,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := svc.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.NextRound(context.Background(), state.Room.Code, RoomActionInput{
			UserID:       state.Room.HostID,
			SessionToken: state.Users[0].SessionToken,
		}); err != nil {
			t.Fatalf("NextRound() error = %v", err)
		}
	}

	next, err := svc.NextRound(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}

	if next.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}

	if next.CurrentRound.RoundNumber != 2 {
		t.Fatalf("round number = %d, want 2", next.CurrentRound.RoundNumber)
	}

	if next.CurrentRound.ID == started.CurrentRound.ID {
		t.Fatal("expected a new round to be created")
	}

	if next.CurrentRound.Status != domain.RoundStatusWriting {
		t.Fatalf("round status = %q, want %q", next.CurrentRound.Status, domain.RoundStatusWriting)
	}
}

func TestStartGameRejectsInvalidSessionToken(t *testing.T) {
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

	if _, err := svc.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: "invalid-token",
	}); err != domain.ErrInvalidSessionToken {
		t.Fatalf("StartGame() error = %v, want %v", err, domain.ErrInvalidSessionToken)
	}
}

func newTestRoomService(cfg config.GameConfig) *RoomService {
	return NewRoomService(
		cfg,
		repository.NewInMemoryGameTypeRepository(),
		repository.NewInMemoryRoomRepository(),
		repository.NewInMemoryUserRepository(),
		repository.NewInMemoryRoundRepository(),
	)
}
