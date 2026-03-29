package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/pkg/utils"
)

// SubmitVoteInput carries vote submission data.
type SubmitVoteInput struct {
	RoundID      string `json:"round_id"`
	UserID       string `json:"user_id"`
	SessionToken string `json:"session_token"`
	StoryID      string `json:"story_id"`
}

// VoteSummary contains per-story vote totals.
type VoteSummary struct {
	StoryID   string `json:"story_id"`
	VoteCount int    `json:"vote_count"`
}

// UserVote describes the user's current vote in a round.
type UserVote struct {
	UserID  string `json:"user_id"`
	RoundID string `json:"round_id"`
	StoryID string `json:"story_id"`
}

// TopStoryResult contains the winning story and its revealed author.
type TopStoryResult struct {
	Story     domain.Story `json:"story"`
	Author    domain.User  `json:"author"`
	VoteCount int          `json:"vote_count"`
}

// VoteService contains vote rules and winner selection.
type VoteService struct {
	roomRepo  repository.RoomRepository
	roundRepo repository.RoundRepository
	userRepo  repository.UserRepository
	storyRepo repository.StoryRepository
	voteRepo  repository.VoteRepository
	lifecycle roundLifecycle
}

// NewVoteService creates a VoteService.
func NewVoteService(
	roomRepo repository.RoomRepository,
	roundRepo repository.RoundRepository,
	userRepo repository.UserRepository,
	storyRepo repository.StoryRepository,
	voteRepo repository.VoteRepository,
) *VoteService {
	return &VoteService{
		roomRepo:  roomRepo,
		roundRepo: roundRepo,
		userRepo:  userRepo,
		storyRepo: storyRepo,
		voteRepo:  voteRepo,
		lifecycle: newRoundLifecycle(roomRepo, roundRepo, nil, nil, nil),
	}
}

// SubmitVote stores one idempotent vote for a round.
func (s *VoteService) SubmitVote(ctx context.Context, input SubmitVoteInput) (domain.Vote, error) {
	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(input.RoundID))
	if err != nil {
		return domain.Vote{}, err
	}

	room, err := s.roomRepo.GetByID(ctx, round.RoomID)
	if err != nil {
		return domain.Vote{}, err
	}
	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return domain.Vote{}, domain.ErrInvalidRoomState
	}

	room, round, err = s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return domain.Vote{}, err
	}

	if room.Status != domain.RoomStatusActive || round.Status != domain.RoundStatusVoting {
		return domain.Vote{}, domain.ErrInvalidRoundState
	}

	user, err := AuthenticateUserSession(ctx, s.userRepo, input.UserID, input.SessionToken)
	if err != nil {
		return domain.Vote{}, err
	}

	story, err := s.storyRepo.GetByID(ctx, strings.TrimSpace(input.StoryID))
	if err != nil {
		return domain.Vote{}, err
	}

	if user.RoomID != round.RoomID || story.RoundID != round.ID {
		return domain.Vote{}, domain.ErrInvalidRoomState
	}

	if story.UserID == user.ID {
		return domain.Vote{}, domain.ErrSelfVote
	}

	if _, err := s.voteRepo.GetByUserAndRound(ctx, user.ID, round.ID); err == nil {
		return domain.Vote{}, domain.ErrVoteAlreadyExists
	} else if !errors.Is(err, domain.ErrVoteNotFound) {
		return domain.Vote{}, err
	}

	voteID, err := utils.GenerateID()
	if err != nil {
		return domain.Vote{}, fmt.Errorf("generate vote id: %w", err)
	}

	vote := domain.Vote{
		ID:        voteID,
		StoryID:   story.ID,
		UserID:    user.ID,
		RoundID:   round.ID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.voteRepo.Create(ctx, vote); err != nil {
		return domain.Vote{}, fmt.Errorf("create vote: %w", err)
	}

	return vote, nil
}

// GetRoundVotes returns aggregated votes for a round.
func (s *VoteService) GetRoundVotes(ctx context.Context, roundID string) ([]VoteSummary, error) {
	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return nil, err
	}

	_, round, err = s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return nil, err
	}

	room, roomErr := s.roomRepo.GetByID(ctx, round.RoomID)
	if roomErr != nil {
		return nil, roomErr
	}

	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return nil, domain.ErrInvalidRoomState
	}

	if round.Status == domain.RoundStatusWriting {
		return nil, domain.ErrInvalidRoundState
	}

	votes, err := s.voteRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return nil, err
	}

	countByStory := make(map[string]int)
	for _, vote := range votes {
		countByStory[vote.StoryID]++
	}

	summaries := make([]VoteSummary, 0, len(countByStory))
	for storyID, count := range countByStory {
		summaries = append(summaries, VoteSummary{
			StoryID:   storyID,
			VoteCount: count,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].VoteCount == summaries[j].VoteCount {
			return summaries[i].StoryID < summaries[j].StoryID
		}
		return summaries[i].VoteCount > summaries[j].VoteCount
	})

	return summaries, nil
}

// GetUserVote returns the user's vote in a round.
func (s *VoteService) GetUserVote(ctx context.Context, userID, roundID, sessionToken string) (UserVote, error) {
	if _, err := AuthenticateUserSession(ctx, s.userRepo, userID, sessionToken); err != nil {
		return UserVote{}, err
	}

	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return UserVote{}, err
	}

	room, err := s.roomRepo.GetByID(ctx, round.RoomID)
	if err != nil {
		return UserVote{}, err
	}

	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return UserVote{}, domain.ErrInvalidRoomState
	}

	vote, err := s.voteRepo.GetByUserAndRound(ctx, strings.TrimSpace(userID), round.ID)
	if err != nil {
		return UserVote{}, err
	}

	return UserVote{
		UserID:  vote.UserID,
		RoundID: vote.RoundID,
		StoryID: vote.StoryID,
	}, nil
}

// GetTopStory returns the round winner with deterministic tie-breaking.
func (s *VoteService) GetTopStory(ctx context.Context, roundID string) (TopStoryResult, error) {
	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return TopStoryResult{}, err
	}

	room, round, err := s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return TopStoryResult{}, err
	}

	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return TopStoryResult{}, domain.ErrInvalidRoomState
	}

	if room.Status == domain.RoomStatusWaiting || round.Status != domain.RoundStatusRevealed {
		return TopStoryResult{}, domain.ErrInvalidRoundState
	}

	stories, err := s.storyRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return TopStoryResult{}, err
	}

	if len(stories) == 0 {
		return TopStoryResult{}, domain.ErrStoryNotFound
	}

	votes, err := s.voteRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return TopStoryResult{}, err
	}

	countByStory := make(map[string]int, len(stories))
	for _, vote := range votes {
		countByStory[vote.StoryID]++
	}

	sort.Slice(stories, func(i, j int) bool {
		leftVotes := countByStory[stories[i].ID]
		rightVotes := countByStory[stories[j].ID]
		if leftVotes == rightVotes {
			return stories[i].CreatedAt.Before(stories[j].CreatedAt)
		}
		return leftVotes > rightVotes
	})

	winner := stories[0]
	winner.IsRevealed = true
	if err := s.storyRepo.Update(ctx, winner); err != nil {
		return TopStoryResult{}, err
	}

	author, err := s.userRepo.GetByID(ctx, winner.UserID)
	if err != nil {
		return TopStoryResult{}, err
	}

	return TopStoryResult{
		Story:     winner,
		Author:    author,
		VoteCount: countByStory[winner.ID],
	}, nil
}
