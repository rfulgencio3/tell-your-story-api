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
)

// SubmitTruthSetVoteInput carries vote submission data for a presented truth set.
type SubmitTruthSetVoteInput struct {
	RoundID                string `json:"round_id"`
	UserID                 string `json:"user_id"`
	SessionToken           string `json:"session_token"`
	TruthSetID             string `json:"truth_set_id"`
	SelectedStatementIndex int    `json:"selected_statement_index"`
}

// ThreeLiesVoteService contains voting-phase logic for three-lies-one-truth.
type ThreeLiesVoteService struct {
	roomRepo         repository.RoomRepository
	roundRepo        repository.RoundRepository
	userRepo         repository.UserRepository
	truthSetRepo     repository.TruthSetRepository
	truthSetVoteRepo repository.TruthSetVoteRepository
	roomScoreRepo    repository.RoomScoreRepository
	lifecycle        roundLifecycle
}

// NewThreeLiesVoteService creates a ThreeLiesVoteService.
func NewThreeLiesVoteService(
	roomRepo repository.RoomRepository,
	roundRepo repository.RoundRepository,
	userRepo repository.UserRepository,
	truthSetRepo repository.TruthSetRepository,
	truthSetVoteRepo repository.TruthSetVoteRepository,
	roomScoreRepo repository.RoomScoreRepository,
) *ThreeLiesVoteService {
	return &ThreeLiesVoteService{
		roomRepo:         roomRepo,
		roundRepo:        roundRepo,
		userRepo:         userRepo,
		truthSetRepo:     truthSetRepo,
		truthSetVoteRepo: truthSetVoteRepo,
		roomScoreRepo:    roomScoreRepo,
		lifecycle:        newRoundLifecycle(roomRepo, roundRepo, truthSetRepo, truthSetVoteRepo, roomScoreRepo),
	}
}

// SubmitVote stores or updates one vote for the active truth set during presentation_voting.
func (s *ThreeLiesVoteService) SubmitVote(ctx context.Context, input SubmitTruthSetVoteInput) (domain.TruthSetVote, bool, error) {
	if input.SelectedStatementIndex < 1 || input.SelectedStatementIndex > 4 {
		return domain.TruthSetVote{}, false, fmt.Errorf("selected_statement_index must be between 1 and 4")
	}

	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(input.RoundID))
	if err != nil {
		return domain.TruthSetVote{}, false, err
	}

	room, round, err := s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return domain.TruthSetVote{}, false, err
	}

	if !domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return domain.TruthSetVote{}, false, domain.ErrInvalidRoomState
	}

	if room.Status != domain.RoomStatusActive || round.Status != domain.RoundStatusPresentationVoting {
		return domain.TruthSetVote{}, false, domain.ErrInvalidRoundState
	}

	now := time.Now().UTC()
	if round.PhaseEndsAt == nil || !now.Before(*round.PhaseEndsAt) {
		return domain.TruthSetVote{}, false, domain.ErrInvalidRoundState
	}

	if strings.TrimSpace(round.ActiveTruthSetID) == "" {
		return domain.TruthSetVote{}, false, domain.ErrActiveTruthSetUnavailable
	}

	user, err := AuthenticateUserSession(ctx, s.userRepo, input.UserID, input.SessionToken)
	if err != nil {
		return domain.TruthSetVote{}, false, err
	}

	if user.RoomID != round.RoomID {
		return domain.TruthSetVote{}, false, domain.ErrUserNotFound
	}

	truthSet, err := s.truthSetRepo.GetByID(ctx, strings.TrimSpace(input.TruthSetID))
	if err != nil {
		return domain.TruthSetVote{}, false, err
	}

	if truthSet.RoundID != round.ID || truthSet.RoomID != room.ID || truthSet.ID != round.ActiveTruthSetID {
		return domain.TruthSetVote{}, false, domain.ErrInvalidRoundState
	}

	if truthSet.AuthorUserID == user.ID {
		return domain.TruthSetVote{}, false, domain.ErrSelfVote
	}

	existing, err := s.truthSetVoteRepo.GetByTruthSetAndUser(ctx, truthSet.ID, user.ID)
	switch {
	case err == nil:
		existing.SelectedStatementIndex = input.SelectedStatementIndex
		existing.UpdatedAt = now
		if updateErr := s.truthSetVoteRepo.Upsert(ctx, existing); updateErr != nil {
			return domain.TruthSetVote{}, false, fmt.Errorf("update truth set vote: %w", updateErr)
		}
		return existing, false, nil
	case !errors.Is(err, domain.ErrTruthSetVoteNotFound):
		return domain.TruthSetVote{}, false, err
	}

	voteID, err := utils.GenerateID()
	if err != nil {
		return domain.TruthSetVote{}, false, fmt.Errorf("generate truth set vote id: %w", err)
	}

	vote := domain.TruthSetVote{
		ID:                     voteID,
		RoomID:                 room.ID,
		RoundID:                round.ID,
		TruthSetID:             truthSet.ID,
		UserID:                 user.ID,
		SelectedStatementIndex: input.SelectedStatementIndex,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.truthSetVoteRepo.Upsert(ctx, vote); err != nil {
		return domain.TruthSetVote{}, false, fmt.Errorf("create truth set vote: %w", err)
	}

	return vote, true, nil
}
