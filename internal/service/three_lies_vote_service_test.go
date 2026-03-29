package service

import (
	"context"
	"testing"
	"time"

	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

func TestGetRoomStateAdvancesWritingToPresentationVotingForThreeLies(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesPresentationFixture(t, true)

	synced, err := fixture.roomService.GetRoomState(context.Background(), fixture.roomCode)
	if err != nil {
		t.Fatalf("GetRoomState() error = %v", err)
	}

	if synced.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}

	if synced.CurrentRound.Status != domain.RoundStatusPresentationVoting {
		t.Fatalf("round status = %q, want %q", synced.CurrentRound.Status, domain.RoundStatusPresentationVoting)
	}

	if synced.CurrentRound.ActiveTruthSetID == "" {
		t.Fatal("active_truth_set_id should not be empty")
	}

	truthSets, err := fixture.truthSetRepo.ListByRoundID(context.Background(), fixture.roundID)
	if err != nil {
		t.Fatalf("ListByRoundID() error = %v", err)
	}

	if len(truthSets) != 2 {
		t.Fatalf("truth set count = %d, want 2", len(truthSets))
	}

	seenOrders := make(map[int]struct{}, len(truthSets))
	for _, truthSet := range truthSets {
		if truthSet.PresentationOrder < 1 || truthSet.PresentationOrder > len(truthSets) {
			t.Fatalf("presentation_order = %d, want between 1 and %d", truthSet.PresentationOrder, len(truthSets))
		}
		seenOrders[truthSet.PresentationOrder] = struct{}{}
	}

	if len(seenOrders) != len(truthSets) {
		t.Fatalf("presentation orders = %d unique values, want %d", len(seenOrders), len(truthSets))
	}

	activeTruthSet, err := fixture.truthSetRepo.GetByID(context.Background(), synced.CurrentRound.ActiveTruthSetID)
	if err != nil {
		t.Fatalf("GetByID(active_truth_set_id) error = %v", err)
	}

	if activeTruthSet.PresentationOrder != 1 {
		t.Fatalf("active truth set presentation_order = %d, want 1", activeTruthSet.PresentationOrder)
	}
}

func TestSubmitTruthSetVoteCreatesAndUpdatesVote(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesPresentationFixture(t, false)

	createdVote, created, err := fixture.voteService.SubmitVote(context.Background(), SubmitTruthSetVoteInput{
		RoundID:                fixture.roundID,
		UserID:                 fixture.guestID,
		SessionToken:           fixture.guestSessionToken,
		TruthSetID:             fixture.activeTruthSetID,
		SelectedStatementIndex: 2,
	})
	if err != nil {
		t.Fatalf("SubmitVote(create) error = %v", err)
	}

	if !created {
		t.Fatal("created = false, want true")
	}

	updatedVote, created, err := fixture.voteService.SubmitVote(context.Background(), SubmitTruthSetVoteInput{
		RoundID:                fixture.roundID,
		UserID:                 fixture.guestID,
		SessionToken:           fixture.guestSessionToken,
		TruthSetID:             fixture.activeTruthSetID,
		SelectedStatementIndex: 4,
	})
	if err != nil {
		t.Fatalf("SubmitVote(update) error = %v", err)
	}

	if created {
		t.Fatal("created = true, want false")
	}

	if updatedVote.ID != createdVote.ID {
		t.Fatalf("vote id = %q, want %q", updatedVote.ID, createdVote.ID)
	}

	if updatedVote.SelectedStatementIndex != 4 {
		t.Fatalf("selected_statement_index = %d, want 4", updatedVote.SelectedStatementIndex)
	}
}

func TestSubmitTruthSetVoteRejectsAuthor(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesPresentationFixture(t, false)

	_, _, err := fixture.voteService.SubmitVote(context.Background(), SubmitTruthSetVoteInput{
		RoundID:                fixture.roundID,
		UserID:                 fixture.hostID,
		SessionToken:           fixture.hostSessionToken,
		TruthSetID:             fixture.activeTruthSetID,
		SelectedStatementIndex: 1,
	})
	if err != domain.ErrSelfVote {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrSelfVote)
	}
}

func TestSubmitTruthSetVoteRejectsExpiredVotingWindow(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesPresentationFixture(t, false)

	expiredAt := time.Now().UTC().Add(-time.Second)
	round, err := fixture.roundRepo.GetByID(context.Background(), fixture.roundID)
	if err != nil {
		t.Fatalf("GetByID(round) error = %v", err)
	}

	round.PhaseEndsAt = &expiredAt
	if err := fixture.roundRepo.Update(context.Background(), round); err != nil {
		t.Fatalf("Update(round) error = %v", err)
	}

	_, _, err = fixture.voteService.SubmitVote(context.Background(), SubmitTruthSetVoteInput{
		RoundID:                fixture.roundID,
		UserID:                 fixture.guestID,
		SessionToken:           fixture.guestSessionToken,
		TruthSetID:             fixture.activeTruthSetID,
		SelectedStatementIndex: 3,
	})
	if err != domain.ErrInvalidRoundState {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrInvalidRoundState)
	}
}

type threeLiesPresentationFixture struct {
	roomService       *RoomService
	truthSetService   *TruthSetService
	voteService       *ThreeLiesVoteService
	roomRepo          repository.RoomRepository
	roundRepo         repository.RoundRepository
	truthSetRepo      repository.TruthSetRepository
	roomCode          string
	roundID           string
	hostID            string
	hostSessionToken  string
	guestID           string
	guestSessionToken string
	activeTruthSetID  string
}

func newThreeLiesPresentationFixture(t *testing.T, submitGuestTruthSet bool) threeLiesPresentationFixture {
	t.Helper()

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()
	roundRepo := repository.NewInMemoryRoundRepository()
	gameTypeRepo := repository.NewInMemoryGameTypeRepository()
	truthSetRepo := repository.NewInMemoryTruthSetRepository()
	truthSetVoteRepo := repository.NewInMemoryTruthSetVoteRepository()
	roomScoreRepo := repository.NewInMemoryRoomScoreRepository()

	roomService := NewRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	}, gameTypeRepo, roomRepo, userRepo, roundRepo, truthSetRepo, truthSetVoteRepo, roomScoreRepo)
	truthSetService := NewTruthSetService(roomRepo, roundRepo, userRepo, truthSetRepo, truthSetVoteRepo, roomScoreRepo)
	voteService := NewThreeLiesVoteService(roomRepo, roundRepo, userRepo, truthSetRepo, truthSetVoteRepo, roomScoreRepo)

	state, err := roomService.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
		GameType:     domain.GameTypeThreeLiesOneTruth,
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joined, err := roomService.JoinRoom(context.Background(), state.Room.Code, JoinRoomInput{
		Nickname: "Guest",
	})
	if err != nil {
		t.Fatalf("JoinRoom() error = %v", err)
	}

	started, err := roomService.StartGame(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	var guestID string
	var guestSessionToken string
	for _, user := range joined.Users {
		if !user.IsHost {
			guestID = user.ID
			guestSessionToken = user.SessionToken
			break
		}
	}

	expiredCountdownAt := time.Now().UTC().Add(-time.Second)
	started.CurrentRound.PhaseEndsAt = &expiredCountdownAt
	if err := roundRepo.Update(context.Background(), *started.CurrentRound); err != nil {
		t.Fatalf("Update(countdown round) error = %v", err)
	}

	writingState, err := roomService.GetRoomState(context.Background(), state.Room.Code)
	if err != nil {
		t.Fatalf("GetRoomState(writing) error = %v", err)
	}

	if _, _, err := truthSetService.SubmitTruthSet(context.Background(), SubmitTruthSetInput{
		RoundID:            writingState.CurrentRound.ID,
		UserID:             state.Room.HostID,
		SessionToken:       state.Users[0].SessionToken,
		Statements:         []string{"Host A", "Host B", "Host C", "Host D"},
		TrueStatementIndex: 2,
	}); err != nil {
		t.Fatalf("SubmitTruthSet(host) error = %v", err)
	}

	if submitGuestTruthSet {
		if _, _, err := truthSetService.SubmitTruthSet(context.Background(), SubmitTruthSetInput{
			RoundID:            writingState.CurrentRound.ID,
			UserID:             guestID,
			SessionToken:       guestSessionToken,
			Statements:         []string{"Guest A", "Guest B", "Guest C", "Guest D"},
			TrueStatementIndex: 3,
		}); err != nil {
			t.Fatalf("SubmitTruthSet(guest) error = %v", err)
		}
	}

	expiredWritingAt := time.Now().UTC().Add(-time.Second)
	writingRound, err := roundRepo.GetByID(context.Background(), writingState.CurrentRound.ID)
	if err != nil {
		t.Fatalf("GetByID(writing round) error = %v", err)
	}
	writingRound.PhaseEndsAt = &expiredWritingAt
	if err := roundRepo.Update(context.Background(), writingRound); err != nil {
		t.Fatalf("Update(writing round) error = %v", err)
	}

	presentationState, err := roomService.GetRoomState(context.Background(), state.Room.Code)
	if err != nil {
		t.Fatalf("GetRoomState(presentation) error = %v", err)
	}

	if presentationState.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil after transition to presentation")
	}

	return threeLiesPresentationFixture{
		roomService:       roomService,
		truthSetService:   truthSetService,
		voteService:       voteService,
		roomRepo:          roomRepo,
		roundRepo:         roundRepo,
		truthSetRepo:      truthSetRepo,
		roomCode:          state.Room.Code,
		roundID:           presentationState.CurrentRound.ID,
		hostID:            state.Room.HostID,
		hostSessionToken:  state.Users[0].SessionToken,
		guestID:           guestID,
		guestSessionToken: guestSessionToken,
		activeTruthSetID:  presentationState.CurrentRound.ActiveTruthSetID,
	}
}
