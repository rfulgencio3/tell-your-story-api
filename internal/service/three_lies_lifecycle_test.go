package service

import (
	"context"
	"testing"
	"time"

	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

func TestThreeLiesLifecycleRevealCommentaryAndFinish(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesLifecycleFixture(t, 1, false)

	if _, _, err := fixture.voteService.SubmitVote(context.Background(), SubmitTruthSetVoteInput{
		RoundID:                fixture.roundID,
		UserID:                 fixture.guestID,
		SessionToken:           fixture.guestSessionToken,
		TruthSetID:             fixture.activeTruthSetID,
		SelectedStatementIndex: 2,
	}); err != nil {
		t.Fatalf("SubmitVote() error = %v", err)
	}

	revealState := fixture.expireAndSync(t)
	if revealState.CurrentRound == nil || revealState.CurrentRound.Status != domain.RoundStatusReveal {
		t.Fatalf("round status = %v, want %v", revealState.CurrentRound.Status, domain.RoundStatusReveal)
	}
	if revealState.ThreeLies == nil || revealState.ThreeLies.Reveal == nil {
		t.Fatal("ThreeLies reveal state should not be nil")
	}
	if revealState.ThreeLies.Reveal.TrueStatementIndex != 2 {
		t.Fatalf("true_statement_index = %d, want 2", revealState.ThreeLies.Reveal.TrueStatementIndex)
	}

	commentaryState := fixture.expireAndSync(t)
	if commentaryState.CurrentRound == nil || commentaryState.CurrentRound.Status != domain.RoundStatusCommentary {
		t.Fatalf("round status = %v, want %v", commentaryState.CurrentRound.Status, domain.RoundStatusCommentary)
	}

	finishedState := fixture.expireAndSync(t)
	if finishedState.Room.Status != domain.RoomStatusFinished {
		t.Fatalf("room status = %q, want %q", finishedState.Room.Status, domain.RoomStatusFinished)
	}
	if finishedState.ThreeLies == nil || len(finishedState.ThreeLies.FinalRanking) != 2 {
		t.Fatal("final ranking should include both players")
	}
	if finishedState.ThreeLies.FinalRanking[0].UserID != fixture.guestID || finishedState.ThreeLies.FinalRanking[0].Score != 12 {
		t.Fatalf("first ranking entry = %+v, want guest with 12 points", finishedState.ThreeLies.FinalRanking[0])
	}
	if finishedState.ThreeLies.FinalRanking[1].UserID != fixture.hostID || finishedState.ThreeLies.FinalRanking[1].Score != 0 {
		t.Fatalf("second ranking entry = %+v, want host with 0 points", finishedState.ThreeLies.FinalRanking[1])
	}
}

func TestThreeLiesLifecycleAdvancesToNextTruthSet(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesLifecycleFixture(t, 1, true)
	firstTruthSetID := fixture.activeTruthSetID

	revealState := fixture.expireAndSync(t)
	if revealState.CurrentRound.Status != domain.RoundStatusReveal {
		t.Fatalf("round status = %v, want %v", revealState.CurrentRound.Status, domain.RoundStatusReveal)
	}

	commentaryState := fixture.expireAndSync(t)
	if commentaryState.CurrentRound.Status != domain.RoundStatusCommentary {
		t.Fatalf("round status = %v, want %v", commentaryState.CurrentRound.Status, domain.RoundStatusCommentary)
	}

	nextPresentation := fixture.expireAndSync(t)
	if nextPresentation.CurrentRound.Status != domain.RoundStatusPresentationVoting {
		t.Fatalf("round status = %v, want %v", nextPresentation.CurrentRound.Status, domain.RoundStatusPresentationVoting)
	}
	if nextPresentation.CurrentRound.ActiveTruthSetID == firstTruthSetID {
		t.Fatal("expected next truth set to become active")
	}
}

func TestThreeLiesLifecycleAdvancesToNextRoundWriting(t *testing.T) {
	t.Parallel()

	fixture := newThreeLiesLifecycleFixture(t, 2, false)

	if _, _, err := fixture.voteService.SubmitVote(context.Background(), SubmitTruthSetVoteInput{
		RoundID:                fixture.roundID,
		UserID:                 fixture.guestID,
		SessionToken:           fixture.guestSessionToken,
		TruthSetID:             fixture.activeTruthSetID,
		SelectedStatementIndex: 1,
	}); err != nil {
		t.Fatalf("SubmitVote() error = %v", err)
	}

	_ = fixture.expireAndSync(t)
	_ = fixture.expireAndSync(t)
	nextRoundState := fixture.expireAndSync(t)

	if nextRoundState.Room.Status != domain.RoomStatusActive {
		t.Fatalf("room status = %q, want %q", nextRoundState.Room.Status, domain.RoomStatusActive)
	}
	if nextRoundState.CurrentRound == nil {
		t.Fatal("CurrentRound should not be nil")
	}
	if nextRoundState.CurrentRound.RoundNumber != 2 {
		t.Fatalf("round number = %d, want 2", nextRoundState.CurrentRound.RoundNumber)
	}
	if nextRoundState.CurrentRound.Status != domain.RoundStatusWriting {
		t.Fatalf("round status = %v, want %v", nextRoundState.CurrentRound.Status, domain.RoundStatusWriting)
	}
}

type threeLiesLifecycleFixture struct {
	roomService       *RoomService
	truthSetService   *TruthSetService
	voteService       *ThreeLiesVoteService
	roundRepo         repository.RoundRepository
	roomCode          string
	roundID           string
	hostID            string
	hostSessionToken  string
	guestID           string
	guestSessionToken string
	activeTruthSetID  string
}

func newThreeLiesLifecycleFixture(t *testing.T, maxRounds int, submitGuestTruthSet bool) threeLiesLifecycleFixture {
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
		MaxRounds:    maxRounds,
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

	expiredAt := time.Now().UTC().Add(-time.Second)
	started.CurrentRound.PhaseEndsAt = &expiredAt
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

	writingRound, err := roundRepo.GetByID(context.Background(), writingState.CurrentRound.ID)
	if err != nil {
		t.Fatalf("GetByID(writing round) error = %v", err)
	}
	writingRound.PhaseEndsAt = &expiredAt
	if err := roundRepo.Update(context.Background(), writingRound); err != nil {
		t.Fatalf("Update(writing round) error = %v", err)
	}

	presentationState, err := roomService.GetRoomState(context.Background(), state.Room.Code)
	if err != nil {
		t.Fatalf("GetRoomState(presentation) error = %v", err)
	}

	return threeLiesLifecycleFixture{
		roomService:       roomService,
		truthSetService:   truthSetService,
		voteService:       voteService,
		roundRepo:         roundRepo,
		roomCode:          state.Room.Code,
		roundID:           presentationState.CurrentRound.ID,
		hostID:            state.Room.HostID,
		hostSessionToken:  state.Users[0].SessionToken,
		guestID:           guestID,
		guestSessionToken: guestSessionToken,
		activeTruthSetID:  presentationState.CurrentRound.ActiveTruthSetID,
	}
}

func (f *threeLiesLifecycleFixture) expireAndSync(t *testing.T) RoomState {
	t.Helper()

	round, err := f.roundRepo.GetByID(context.Background(), f.roundID)
	if err != nil {
		t.Fatalf("GetByID(round) error = %v", err)
	}

	expiredAt := time.Now().UTC().Add(-time.Second)
	round.PhaseEndsAt = &expiredAt
	if err := f.roundRepo.Update(context.Background(), round); err != nil {
		t.Fatalf("Update(round) error = %v", err)
	}

	state, err := f.roomService.GetRoomState(context.Background(), f.roomCode)
	if err != nil {
		t.Fatalf("GetRoomState() error = %v", err)
	}

	if state.CurrentRound != nil {
		f.roundID = state.CurrentRound.ID
		f.activeTruthSetID = state.CurrentRound.ActiveTruthSetID
	}

	return state
}
