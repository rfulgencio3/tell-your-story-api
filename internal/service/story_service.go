package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/pkg/utils"
	"github.com/tell-your-story/backend/pkg/validator"
)

var defaultBadWords = map[string]struct{}{
	"idiot":  {},
	"stupid": {},
	"hate":   {},
}

// SubmitStoryInput carries story submission data.
type SubmitStoryInput struct {
	RoundID      string `json:"round_id"`
	UserID       string `json:"user_id"`
	SessionToken string `json:"session_token"`
	Title        string `json:"title"`
	Body         string `json:"body"`
}

// StoryCard is the public story payload returned to clients.
type StoryCard struct {
	ID         string    `json:"id"`
	RoundID    string    `json:"round_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	IsRevealed bool      `json:"is_revealed"`
	VoteCount  int       `json:"vote_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// StoryService contains story submission logic.
type StoryService struct {
	roomRepo  repository.RoomRepository
	roundRepo repository.RoundRepository
	userRepo  repository.UserRepository
	storyRepo repository.StoryRepository
	voteRepo  repository.VoteRepository
	lifecycle roundLifecycle
}

// NewStoryService creates a StoryService.
func NewStoryService(
	roomRepo repository.RoomRepository,
	roundRepo repository.RoundRepository,
	userRepo repository.UserRepository,
	storyRepo repository.StoryRepository,
	voteRepo repository.VoteRepository,
) *StoryService {
	return &StoryService{
		roomRepo:  roomRepo,
		roundRepo: roundRepo,
		userRepo:  userRepo,
		storyRepo: storyRepo,
		voteRepo:  voteRepo,
		lifecycle: newRoundLifecycle(roomRepo, roundRepo, nil),
	}
}

// SubmitStory creates a story for a round.
func (s *StoryService) SubmitStory(ctx context.Context, input SubmitStoryInput) (domain.Story, error) {
	if err := validator.Story(input.Title, input.Body); err != nil {
		return domain.Story{}, err
	}

	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(input.RoundID))
	if err != nil {
		return domain.Story{}, err
	}

	room, err := s.roomRepo.GetByID(ctx, round.RoomID)
	if err != nil {
		return domain.Story{}, err
	}
	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return domain.Story{}, domain.ErrInvalidRoomState
	}

	room, round, err = s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return domain.Story{}, err
	}

	if room.Status != domain.RoomStatusActive || round.Status != domain.RoundStatusWriting {
		return domain.Story{}, domain.ErrInvalidRoundState
	}

	user, err := AuthenticateUserSession(ctx, s.userRepo, input.UserID, input.SessionToken)
	if err != nil {
		return domain.Story{}, err
	}

	if user.RoomID != round.RoomID {
		return domain.Story{}, domain.ErrUserNotFound
	}

	if _, err := s.storyRepo.GetByUserAndRound(ctx, user.ID, round.ID); err == nil {
		return domain.Story{}, domain.ErrStoryAlreadySubmitted
	} else if !errors.Is(err, domain.ErrStoryNotFound) {
		return domain.Story{}, err
	}

	storyID, err := utils.GenerateID()
	if err != nil {
		return domain.Story{}, fmt.Errorf("generate story id: %w", err)
	}

	story := domain.Story{
		ID:         storyID,
		RoundID:    round.ID,
		UserID:     user.ID,
		Title:      utils.SanitizeText(strings.TrimSpace(input.Title), defaultBadWords),
		Body:       utils.SanitizeText(strings.TrimSpace(input.Body), defaultBadWords),
		IsRevealed: false,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.storyRepo.Create(ctx, story); err != nil {
		return domain.Story{}, fmt.Errorf("create story: %w", err)
	}

	return story, nil
}

// GetRoundStories returns anonymous, shuffled stories for a round.
func (s *StoryService) GetRoundStories(ctx context.Context, roundID string) ([]StoryCard, error) {
	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return nil, err
	}

	room, err := s.roomRepo.GetByID(ctx, round.RoomID)
	if err != nil {
		return nil, err
	}
	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return nil, domain.ErrInvalidRoomState
	}

	room, round, err = s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return nil, err
	}

	if room.Status == domain.RoomStatusWaiting || round.Status == domain.RoundStatusWriting {
		return nil, domain.ErrInvalidRoundState
	}

	stories, err := s.storyRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return nil, err
	}

	votes, err := s.voteRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return nil, err
	}

	voteCountByStory := make(map[string]int, len(stories))
	for _, vote := range votes {
		voteCountByStory[vote.StoryID]++
	}

	shuffled := utils.ShuffleStories(stories)
	cards := make([]StoryCard, 0, len(shuffled))
	for _, story := range shuffled {
		cards = append(cards, StoryCard{
			ID:         story.ID,
			RoundID:    story.RoundID,
			Title:      story.Title,
			Body:       story.Body,
			IsRevealed: story.IsRevealed,
			VoteCount:  voteCountByStory[story.ID],
			CreatedAt:  story.CreatedAt,
		})
	}

	return cards, nil
}
