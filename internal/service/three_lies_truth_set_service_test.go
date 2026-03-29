package service

import (
	"context"
	"testing"
	"time"

	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

func TestSubmitTruthSetCreatesNewSet(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesWritingFixture(t)

	truthSet, created, err := fixture.truthSetService.SubmitTruthSet(context.Background(), SubmitTruthSetInput{
		RoundID:            fixture.roundID,
		UserID:             fixture.hostID,
		SessionToken:       fixture.hostSessionToken,
		Statements:         []string{"A", "B", "C", "D"},
		TrueStatementIndex: 2,
	})
	if err != nil {
		t.Fatalf("SubmitTruthSet() error = %v", err)
	}

	if !created {
		t.Fatal("created = false, want true")
	}

	if truthSet.AuthorUserID != fixture.hostID {
		t.Fatalf("author_user_id = %q, want %q", truthSet.AuthorUserID, fixture.hostID)
	}

	if len(truthSet.Statements) != 4 {
		t.Fatalf("statements count = %d, want 4", len(truthSet.Statements))
	}
}

func TestSubmitTruthSetUpdatesExistingSet(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesWritingFixture(t)

	first, created, err := fixture.truthSetService.SubmitTruthSet(context.Background(), SubmitTruthSetInput{
		RoundID:            fixture.roundID,
		UserID:             fixture.hostID,
		SessionToken:       fixture.hostSessionToken,
		Statements:         []string{"A", "B", "C", "D"},
		TrueStatementIndex: 2,
	})
	if err != nil {
		t.Fatalf("SubmitTruthSet(first) error = %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}

	updated, created, err := fixture.truthSetService.SubmitTruthSet(context.Background(), SubmitTruthSetInput{
		RoundID:            fixture.roundID,
		UserID:             fixture.hostID,
		SessionToken:       fixture.hostSessionToken,
		Statements:         []string{"W", "X", "Y", "Z"},
		TrueStatementIndex: 4,
	})
	if err != nil {
		t.Fatalf("SubmitTruthSet(update) error = %v", err)
	}

	if created {
		t.Fatal("created = true, want false")
	}

	if updated.ID != first.ID {
		t.Fatalf("truth_set id = %q, want %q", updated.ID, first.ID)
	}

	if updated.TrueStatementIndex != 4 {
		t.Fatalf("true_statement_index = %d, want 4", updated.TrueStatementIndex)
	}

	if updated.Statements[3].Content != "Z" {
		t.Fatalf("statement[4] = %q, want %q", updated.Statements[3].Content, "Z")
	}
}

func TestSubmitTruthSetRejectsTellYourStoryRoom(t *testing.T) {
	t.Parallel()

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()
	roundRepo := repository.NewInMemoryRoundRepository()
	gameTypeRepo := repository.NewInMemoryGameTypeRepository()
	truthSetRepo := repository.NewInMemoryTruthSetRepository()

	roomService := NewRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	}, gameTypeRepo, roomRepo, userRepo, roundRepo)
	truthSetService := NewTruthSetService(roomRepo, roundRepo, userRepo, truthSetRepo)

	state, err := roomService.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := roomService.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	_, _, err = truthSetService.SubmitTruthSet(context.Background(), SubmitTruthSetInput{
		RoundID:            started.CurrentRound.ID,
		UserID:             state.Room.HostID,
		SessionToken:       state.Users[0].SessionToken,
		Statements:         []string{"A", "B", "C", "D"},
		TrueStatementIndex: 1,
	})
	if err != domain.ErrInvalidRoomState {
		t.Fatalf("SubmitTruthSet() error = %v, want %v", err, domain.ErrInvalidRoomState)
	}
}

type threeLiesWritingFixture struct {
	roomService      *RoomService
	truthSetService  *TruthSetService
	roundRepo        repository.RoundRepository
	roomCode         string
	roundID          string
	hostID           string
	hostSessionToken string
}

func newThreeLiesWritingFixture(t *testing.T) threeLiesWritingFixture {
	t.Helper()

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()
	roundRepo := repository.NewInMemoryRoundRepository()
	gameTypeRepo := repository.NewInMemoryGameTypeRepository()
	truthSetRepo := repository.NewInMemoryTruthSetRepository()

	roomService := NewRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	}, gameTypeRepo, roomRepo, userRepo, roundRepo)
	truthSetService := NewTruthSetService(roomRepo, roundRepo, userRepo, truthSetRepo)

	state, err := roomService.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		GameType:     domain.GameTypeThreeLiesOneTruth,
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	started, err := roomService.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	expiredAt := time.Now().UTC().Add(-time.Second)
	started.CurrentRound.PhaseEndsAt = &expiredAt
	if err := roundRepo.Update(context.Background(), *started.CurrentRound); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	synced, err := roomService.GetRoomState(context.Background(), state.Room.Code)
	if err != nil {
		t.Fatalf("GetRoomState() error = %v", err)
	}

	return threeLiesWritingFixture{
		roomService:      roomService,
		truthSetService:  truthSetService,
		roundRepo:        roundRepo,
		roomCode:         state.Room.Code,
		roundID:          synced.CurrentRound.ID,
		hostID:           state.Room.HostID,
		hostSessionToken: state.Users[0].SessionToken,
	}
}
