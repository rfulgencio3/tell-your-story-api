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
		RoundID: fixture.roundID,
		UserID:  fixture.guestID,
		StoryID: fixture.hostStoryID,
	})
	if err != nil {
		t.Fatalf("SubmitVote() unexpected error = %v", err)
	}

	if firstVote.ID == "" {
		t.Fatal("vote id should not be empty")
	}

	if _, err := fixture.voteService.SubmitVote(context.Background(), SubmitVoteInput{
		RoundID: fixture.roundID,
		UserID:  fixture.guestID,
		StoryID: fixture.hostStoryID,
	}); err != domain.ErrVoteAlreadyExists {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrVoteAlreadyExists)
	}
}

func TestSubmitVoteRejectsSelfVote(t *testing.T) {
	t.Parallel()

	fixture := newGameplayFixture(t)

	if _, err := fixture.voteService.SubmitVote(context.Background(), SubmitVoteInput{
		RoundID: fixture.roundID,
		UserID:  fixture.hostID,
		StoryID: fixture.hostStoryID,
	}); err != domain.ErrSelfVote {
		t.Fatalf("SubmitVote() error = %v, want %v", err, domain.ErrSelfVote)
	}
}

type gameplayFixture struct {
	roomService  *RoomService
	storyService *StoryService
	voteService  *VoteService
	hostID       string
	guestID      string
	roundID      string
	hostStoryID  string
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
	storyService := NewStoryService(roundRepo, userRepo, storyRepo, voteRepo)
	voteService := NewVoteService(roundRepo, userRepo, storyRepo, voteRepo)

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
		UserID: state.Room.HostID,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	var guestID string
	for _, user := range joined.Users {
		if !user.IsHost {
			guestID = user.ID
			break
		}
	}

	hostStory, err := storyService.SubmitStory(context.Background(), SubmitStoryInput{
		RoundID: started.CurrentRound.ID,
		UserID:  state.Room.HostID,
		Title:   "Host story",
		Body:    "A fun fact",
	})
	if err != nil {
		t.Fatalf("SubmitStory(host) error = %v", err)
	}

	if _, err := storyService.SubmitStory(context.Background(), SubmitStoryInput{
		RoundID: started.CurrentRound.ID,
		UserID:  guestID,
		Title:   "Guest story",
		Body:    "Another fun fact",
	}); err != nil {
		t.Fatalf("SubmitStory(guest) error = %v", err)
	}

	return gameplayFixture{
		roomService:  roomService,
		storyService: storyService,
		voteService:  voteService,
		hostID:       state.Room.HostID,
		guestID:      guestID,
		roundID:      started.CurrentRound.ID,
		hostStoryID:  hostStory.ID,
	}
}
