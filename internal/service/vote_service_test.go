package service

import (
	"context"
	"testing"
	"time"

	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

func TestSubmitVoteRejectsDuplicateVotes(t *testing.T) {
	t.Parallel()

	fixture := newGameplayFixture(t)

	firstVote, err := fixture.voteService.SubmitVote(context.Background(), SubmitVoteInput{
		RoundID:      fixture.roundID,
		UserID:       fixture.guestID,
		SessionToken: fixture.guestSessionToken,
		StoryID:      fixture.hostStoryID,
	})
	if err != nil {
		t.Fatalf("SubmitVote() unexpected error = %v", err)
	}

	if firstVote.ID == "" {
		t.Fatal("vote id should not be empty")
	}

	if _, err := fixture.voteService.SubmitVote(context.Background(), SubmitVoteInput{
		RoundID:      fixture.roundID,
		UserID:       fixture.guestID,
		SessionToken: fixture.guestSessionToken,
		StoryID:      fixture.hostStoryID,
	}); err != domain.ErrVoteAlreadyExists {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrVoteAlreadyExists)
	}
}

func TestSubmitVoteRejectsSelfVote(t *testing.T) {
	t.Parallel()

	fixture := newGameplayFixture(t)

	if _, err := fixture.voteService.SubmitVote(context.Background(), SubmitVoteInput{
		RoundID:      fixture.roundID,
		UserID:       fixture.hostID,
		SessionToken: fixture.hostSessionToken,
		StoryID:      fixture.hostStoryID,
	}); err != domain.ErrSelfVote {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrSelfVote)
	}
}

func TestSubmitStoryRejectsWhenRoundNotWriting(t *testing.T) {
	t.Parallel()

	fixture := newGameplayFixture(t)

	if _, err := fixture.storyService.SubmitStory(context.Background(), SubmitStoryInput{
		RoundID:      fixture.roundID,
		UserID:       fixture.guestID,
		SessionToken: fixture.guestSessionToken,
		Title:        "Late story",
		Body:         "This should fail",
	}); err != domain.ErrInvalidRoundState {
		t.Fatalf("SubmitStory() error = %v, want %v", err, domain.ErrInvalidRoundState)
	}
}

func TestSubmitVoteRejectsInvalidSessionToken(t *testing.T) {
	t.Parallel()

	fixture := newGameplayFixture(t)

	if _, err := fixture.voteService.SubmitVote(context.Background(), SubmitVoteInput{
		RoundID:      fixture.roundID,
		UserID:       fixture.guestID,
		SessionToken: "invalid-token",
		StoryID:      fixture.hostStoryID,
	}); err != domain.ErrInvalidSessionToken {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrInvalidSessionToken)
	}
}

type gameplayFixture struct {
	roomService       *RoomService
	storyService      *StoryService
	voteService       *VoteService
	hostID            string
	hostSessionToken  string
	guestID           string
	guestSessionToken string
	roundID           string
	hostStoryID       string
}

func newGameplayFixture(t *testing.T) gameplayFixture {
	t.Helper()

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()
	roundRepo := repository.NewInMemoryRoundRepository()
	storyRepo := repository.NewInMemoryStoryRepository()
	voteRepo := repository.NewInMemoryVoteRepository()

	roomService := NewRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	}, roomRepo, userRepo, roundRepo)
	storyService := NewStoryService(roomRepo, roundRepo, userRepo, storyRepo, voteRepo)
	voteService := NewVoteService(roomRepo, roundRepo, userRepo, storyRepo, voteRepo)

	state, err := roomService.CreateRoom(context.Background(), CreateRoomInput{
		HostNickname: "Host",
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

	hostStory, err := storyService.SubmitStory(context.Background(), SubmitStoryInput{
		RoundID:      started.CurrentRound.ID,
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
		Title:        "Host story",
		Body:         "A fun fact",
	})
	if err != nil {
		t.Fatalf("SubmitStory(host) error = %v", err)
	}

	if _, err := storyService.SubmitStory(context.Background(), SubmitStoryInput{
		RoundID:      started.CurrentRound.ID,
		UserID:       guestID,
		SessionToken: guestSessionToken,
		Title:        "Guest story",
		Body:         "Another fun fact",
	}); err != nil {
		t.Fatalf("SubmitStory(guest) error = %v", err)
	}

	if _, err := roomService.NextRound(context.Background(), state.Room.Code, RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	}); err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}

	return gameplayFixture{
		roomService:       roomService,
		storyService:      storyService,
		voteService:       voteService,
		hostID:            state.Room.HostID,
		hostSessionToken:  state.Users[0].SessionToken,
		guestID:           guestID,
		guestSessionToken: guestSessionToken,
		roundID:           started.CurrentRound.ID,
		hostStoryID:       hostStory.ID,
	}
}
