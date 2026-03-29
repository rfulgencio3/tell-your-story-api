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

// SubmitTruthSetInput carries truth set submission data for the three-lies mode.
type SubmitTruthSetInput struct {
	RoundID            string   `json:"round_id"`
	UserID             string   `json:"user_id"`
	SessionToken       string   `json:"session_token"`
	Statements         []string `json:"statements"`
	TrueStatementIndex int      `json:"true_statement_index"`
}

// TruthSetService contains writing-phase logic for three-lies-one-truth.
type TruthSetService struct {
	roomRepo         repository.RoomRepository
	roundRepo        repository.RoundRepository
	userRepo         repository.UserRepository
	truthSetRepo     repository.TruthSetRepository
	truthSetVoteRepo repository.TruthSetVoteRepository
	roomScoreRepo    repository.RoomScoreRepository
	lifecycle        roundLifecycle
}

// NewTruthSetService creates a TruthSetService.
func NewTruthSetService(
	roomRepo repository.RoomRepository,
	roundRepo repository.RoundRepository,
	userRepo repository.UserRepository,
	truthSetRepo repository.TruthSetRepository,
	truthSetVoteRepo repository.TruthSetVoteRepository,
	roomScoreRepo repository.RoomScoreRepository,
) *TruthSetService {
	return &TruthSetService{
		roomRepo:         roomRepo,
		roundRepo:        roundRepo,
		userRepo:         userRepo,
		truthSetRepo:     truthSetRepo,
		truthSetVoteRepo: truthSetVoteRepo,
		roomScoreRepo:    roomScoreRepo,
		lifecycle:        newRoundLifecycle(roomRepo, roundRepo, truthSetRepo, truthSetVoteRepo, roomScoreRepo),
	}
}

// SubmitTruthSet creates or updates the caller's truth set during writing.
func (s *TruthSetService) SubmitTruthSet(ctx context.Context, input SubmitTruthSetInput) (domain.TruthSet, bool, error) {
	if err := validator.TruthSet(input.Statements, input.TrueStatementIndex); err != nil {
		return domain.TruthSet{}, false, err
	}

	round, err := s.roundRepo.GetByID(ctx, strings.TrimSpace(input.RoundID))
	if err != nil {
		return domain.TruthSet{}, false, err
	}

	room, round, err := s.lifecycle.SyncRound(ctx, round)
	if err != nil {
		return domain.TruthSet{}, false, err
	}

	if !domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return domain.TruthSet{}, false, domain.ErrInvalidRoomState
	}

	if room.Status != domain.RoomStatusActive || round.Status != domain.RoundStatusWriting {
		return domain.TruthSet{}, false, domain.ErrInvalidRoundState
	}

	user, err := AuthenticateUserSession(ctx, s.userRepo, input.UserID, input.SessionToken)
	if err != nil {
		return domain.TruthSet{}, false, err
	}

	if user.RoomID != round.RoomID {
		return domain.TruthSet{}, false, domain.ErrUserNotFound
	}

	existing, err := s.truthSetRepo.GetByAuthorAndRound(ctx, user.ID, round.ID)
	if err == nil {
		updatedAt := time.Now().UTC()
		statements, buildErr := buildTruthSetStatements(existing.ID, input.Statements, existing.CreatedAt, updatedAt)
		if buildErr != nil {
			return domain.TruthSet{}, false, buildErr
		}
		updated := domain.TruthSet{
			ID:                 existing.ID,
			RoomID:             existing.RoomID,
			RoundID:            existing.RoundID,
			AuthorUserID:       existing.AuthorUserID,
			PresentationOrder:  existing.PresentationOrder,
			TrueStatementIndex: input.TrueStatementIndex,
			CommentaryText:     existing.CommentaryText,
			CreatedAt:          existing.CreatedAt,
			UpdatedAt:          updatedAt,
			Statements:         statements,
		}
		if updateErr := s.truthSetRepo.Update(ctx, updated); updateErr != nil {
			return domain.TruthSet{}, false, fmt.Errorf("update truth set: %w", updateErr)
		}
		return updated, false, nil
	}
	if !errors.Is(err, domain.ErrTruthSetNotFound) {
		return domain.TruthSet{}, false, err
	}

	truthSetID, err := utils.GenerateID()
	if err != nil {
		return domain.TruthSet{}, false, fmt.Errorf("generate truth set id: %w", err)
	}

	now := time.Now().UTC()
	statements, err := buildTruthSetStatements(truthSetID, input.Statements, now, now)
	if err != nil {
		return domain.TruthSet{}, false, err
	}
	truthSet := domain.TruthSet{
		ID:                 truthSetID,
		RoomID:             room.ID,
		RoundID:            round.ID,
		AuthorUserID:       user.ID,
		PresentationOrder:  0,
		TrueStatementIndex: input.TrueStatementIndex,
		CreatedAt:          now,
		UpdatedAt:          now,
		Statements:         statements,
	}

	if err := s.truthSetRepo.Create(ctx, truthSet); err != nil {
		return domain.TruthSet{}, false, fmt.Errorf("create truth set: %w", err)
	}

	return truthSet, true, nil
}

func buildTruthSetStatements(truthSetID string, statements []string, createdAt, updatedAt time.Time) ([]domain.TruthSetStatement, error) {
	items := make([]domain.TruthSetStatement, 0, len(statements))
	for index, content := range statements {
		statementID, err := utils.GenerateID()
		if err != nil {
			return nil, fmt.Errorf("generate truth set statement id: %w", err)
		}
		items = append(items, domain.TruthSetStatement{
			ID:             statementID,
			TruthSetID:     truthSetID,
			StatementIndex: index + 1,
			Content:        strings.TrimSpace(content),
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}

	return items, nil
}
