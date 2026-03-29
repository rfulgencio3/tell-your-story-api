package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/pkg/utils"
)

const defaultVotingPhaseDuration = time.Minute
const defaultCountdownPhaseDuration = 3 * time.Second
const defaultRevealPhaseDuration = 5 * time.Second
const defaultCommentaryPhaseDuration = 15 * time.Second

type roundLifecycle struct {
	roomRepo         repository.RoomRepository
	roundRepo        repository.RoundRepository
	truthSetRepo     repository.TruthSetRepository
	truthSetVoteRepo repository.TruthSetVoteRepository
	roomScoreRepo    repository.RoomScoreRepository
}

func newRoundLifecycle(
	roomRepo repository.RoomRepository,
	roundRepo repository.RoundRepository,
	truthSetRepo repository.TruthSetRepository,
	truthSetVoteRepo repository.TruthSetVoteRepository,
	roomScoreRepo repository.RoomScoreRepository,
) roundLifecycle {
	return roundLifecycle{
		roomRepo:         roomRepo,
		roundRepo:        roundRepo,
		truthSetRepo:     truthSetRepo,
		truthSetVoteRepo: truthSetVoteRepo,
		roomScoreRepo:    roomScoreRepo,
	}
}

func (l roundLifecycle) SyncRound(ctx context.Context, round domain.Round) (domain.Room, domain.Round, error) {
	room, err := l.roomRepo.GetByID(ctx, round.RoomID)
	if err != nil {
		return domain.Room{}, domain.Round{}, err
	}

	now := time.Now().UTC()
	if now.After(room.ExpiresAt) && room.Status != domain.RoomStatusExpired {
		room.Status = domain.RoomStatusExpired
		if err := l.roomRepo.Update(ctx, room); err != nil {
			return domain.Room{}, domain.Round{}, fmt.Errorf("expire room: %w", err)
		}
		return room, round, domain.ErrRoomExpired
	}

	if room.Status == domain.RoomStatusExpired {
		return room, round, domain.ErrRoomExpired
	}

	if room.Status != domain.RoomStatusActive {
		return room, round, nil
	}

	if round.PausedAt != nil || round.PhaseEndsAt == nil {
		return room, round, nil
	}

	if now.Before(*round.PhaseEndsAt) {
		return room, round, nil
	}

	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		switch round.Status {
		case domain.RoundStatusCountdown:
			updatedRound, err := l.advanceToWriting(ctx, round, now, room.TimePerRound)
			if err != nil {
				return domain.Room{}, domain.Round{}, err
			}
			return room, updatedRound, nil
		case domain.RoundStatusWriting:
			updatedRound, err := l.advanceToPresentationVoting(ctx, round, now)
			if err != nil {
				return domain.Room{}, domain.Round{}, err
			}
			return room, updatedRound, nil
		case domain.RoundStatusPresentationVoting:
			updatedRound, err := l.advanceToReveal(ctx, room, round, now)
			if err != nil {
				return domain.Room{}, domain.Round{}, err
			}
			return room, updatedRound, nil
		case domain.RoundStatusReveal:
			updatedRound, err := l.advanceToCommentary(ctx, round, now)
			if err != nil {
				return domain.Room{}, domain.Round{}, err
			}
			return room, updatedRound, nil
		case domain.RoundStatusCommentary:
			updatedRoom, updatedRound, err := l.advanceAfterCommentary(ctx, room, round, now)
			if err != nil {
				return domain.Room{}, domain.Round{}, err
			}
			return updatedRoom, updatedRound, nil
		default:
			return room, round, nil
		}
	}

	switch round.Status {
	case domain.RoundStatusWriting:
		updatedRound, err := l.advanceToVoting(ctx, round, now)
		if err != nil {
			return domain.Room{}, domain.Round{}, err
		}
		return room, updatedRound, nil
	case domain.RoundStatusVoting:
		updatedRound, err := l.advanceToRevealed(ctx, round, now)
		if err != nil {
			return domain.Room{}, domain.Round{}, err
		}
		return room, updatedRound, nil
	default:
		return room, round, nil
	}
}

func (l roundLifecycle) advanceToWriting(ctx context.Context, round domain.Round, now time.Time, durationSeconds int) (domain.Round, error) {
	phaseEndsAt := now.Add(time.Duration(durationSeconds) * time.Second)
	round.Status = domain.RoundStatusWriting
	round.PhaseEndsAt = &phaseEndsAt
	round.PausedAt = nil

	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("advance round to writing: %w", err)
	}

	return round, nil
}

func (l roundLifecycle) advanceToPresentationVoting(ctx context.Context, round domain.Round, now time.Time) (domain.Round, error) {
	if l.truthSetRepo == nil {
		return domain.Round{}, fmt.Errorf("advance round to presentation voting: truth set repository is required")
	}

	truthSets, err := l.truthSetRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return domain.Round{}, fmt.Errorf("list truth sets for presentation: %w", err)
	}
	if len(truthSets) == 0 {
		return domain.Round{}, domain.ErrActiveTruthSetUnavailable
	}

	if !presentationOrderAssigned(truthSets) {
		random := rand.New(rand.NewSource(now.UnixNano()))
		random.Shuffle(len(truthSets), func(i, j int) {
			truthSets[i], truthSets[j] = truthSets[j], truthSets[i]
		})

		for index := range truthSets {
			truthSets[index].PresentationOrder = index + 1
			truthSets[index].UpdatedAt = now
			if updateErr := l.truthSetRepo.Update(ctx, truthSets[index]); updateErr != nil {
				return domain.Round{}, fmt.Errorf("persist presentation order: %w", updateErr)
			}
		}
	}

	firstTruthSet, err := firstPresentationTruthSet(ctx, l.truthSetRepo, round.ID)
	if err != nil {
		return domain.Round{}, err
	}

	phaseEndsAt := now.Add(defaultVotingPhaseDuration)
	round.Status = domain.RoundStatusPresentationVoting
	round.ActiveTruthSetID = firstTruthSet.ID
	round.PhaseEndsAt = &phaseEndsAt
	round.PausedAt = nil

	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("advance round to presentation voting: %w", err)
	}

	return round, nil
}

func (l roundLifecycle) advanceToReveal(ctx context.Context, room domain.Room, round domain.Round, now time.Time) (domain.Round, error) {
	if l.truthSetRepo == nil || l.truthSetVoteRepo == nil || l.roomScoreRepo == nil {
		return domain.Round{}, fmt.Errorf("advance round to reveal: required repositories are missing")
	}
	if round.ActiveTruthSetID == "" {
		return domain.Round{}, domain.ErrActiveTruthSetUnavailable
	}

	truthSet, err := l.truthSetRepo.GetByID(ctx, round.ActiveTruthSetID)
	if err != nil {
		return domain.Round{}, err
	}

	if truthSet.ScoredAt == nil {
		votes, err := l.truthSetVoteRepo.ListByTruthSetID(ctx, truthSet.ID)
		if err != nil {
			return domain.Round{}, fmt.Errorf("list truth set votes for scoring: %w", err)
		}

		playerPoints, authorPoints, _ := calculateThreeLiesScoreDelta(truthSet, votes)
		for userID, points := range playerPoints {
			if points == 0 {
				continue
			}
			if _, err := l.roomScoreRepo.Increment(ctx, room.ID, userID, points, scoreUpdatedAt(now)); err != nil {
				return domain.Round{}, fmt.Errorf("increment player room score: %w", err)
			}
		}
		if authorPoints > 0 {
			if _, err := l.roomScoreRepo.Increment(ctx, room.ID, truthSet.AuthorUserID, authorPoints, scoreUpdatedAt(now)); err != nil {
				return domain.Round{}, fmt.Errorf("increment author room score: %w", err)
			}
		}

		truthSet.ScoredAt = &now
		truthSet.UpdatedAt = now
		if err := l.truthSetRepo.Update(ctx, truthSet); err != nil {
			return domain.Round{}, fmt.Errorf("mark truth set as scored: %w", err)
		}
	}

	phaseEndsAt := now.Add(defaultRevealPhaseDuration)
	round.Status = domain.RoundStatusReveal
	round.PhaseEndsAt = &phaseEndsAt
	round.PausedAt = nil

	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("advance round to reveal: %w", err)
	}

	return round, nil
}

func (l roundLifecycle) advanceToCommentary(ctx context.Context, round domain.Round, now time.Time) (domain.Round, error) {
	phaseEndsAt := now.Add(defaultCommentaryPhaseDuration)
	round.Status = domain.RoundStatusCommentary
	round.PhaseEndsAt = &phaseEndsAt
	round.PausedAt = nil

	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("advance round to commentary: %w", err)
	}

	return round, nil
}

func (l roundLifecycle) advanceAfterCommentary(ctx context.Context, room domain.Room, round domain.Round, now time.Time) (domain.Room, domain.Round, error) {
	if l.truthSetRepo == nil {
		return domain.Room{}, domain.Round{}, fmt.Errorf("advance round after commentary: truth set repository is required")
	}

	truthSets, err := l.truthSetRepo.ListByRoundID(ctx, round.ID)
	if err != nil {
		return domain.Room{}, domain.Round{}, fmt.Errorf("list truth sets after commentary: %w", err)
	}
	if len(truthSets) == 0 {
		return domain.Room{}, domain.Round{}, domain.ErrActiveTruthSetUnavailable
	}

	nextTruthSet, hasNext := nextPresentationTruthSet(truthSets, round.ActiveTruthSetID)
	if hasNext {
		phaseEndsAt := now.Add(defaultVotingPhaseDuration)
		round.Status = domain.RoundStatusPresentationVoting
		round.ActiveTruthSetID = nextTruthSet.ID
		round.PhaseEndsAt = &phaseEndsAt
		round.PausedAt = nil
		round.CompletedAt = nil
		if err := l.roundRepo.Update(ctx, round); err != nil {
			return domain.Room{}, domain.Round{}, fmt.Errorf("advance round to next presentation voting: %w", err)
		}
		return room, round, nil
	}

	round.Status = domain.RoundStatusFinished
	round.PhaseEndsAt = nil
	round.PausedAt = nil
	round.CompletedAt = &now
	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Room{}, domain.Round{}, fmt.Errorf("finish three-lies round: %w", err)
	}

	if round.RoundNumber < room.MaxRounds {
		nextRound, err := l.newThreeLiesWritingRound(room, round.RoundNumber+1, now)
		if err != nil {
			return domain.Room{}, domain.Round{}, err
		}
		if err := l.roundRepo.Create(ctx, nextRound); err != nil {
			return domain.Room{}, domain.Round{}, fmt.Errorf("create next three-lies round: %w", err)
		}
		return room, nextRound, nil
	}

	room.Status = domain.RoomStatusFinished
	if err := l.roomRepo.Update(ctx, room); err != nil {
		return domain.Room{}, domain.Round{}, fmt.Errorf("finish room after three-lies commentary: %w", err)
	}

	return room, round, nil
}

func (l roundLifecycle) advanceToVoting(ctx context.Context, round domain.Round, now time.Time) (domain.Round, error) {
	phaseEndsAt := now.Add(defaultVotingPhaseDuration)
	round.Status = domain.RoundStatusVoting
	round.PhaseEndsAt = &phaseEndsAt
	round.PausedAt = nil

	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("advance round to voting: %w", err)
	}

	return round, nil
}

func (l roundLifecycle) advanceToRevealed(ctx context.Context, round domain.Round, now time.Time) (domain.Round, error) {
	round.Status = domain.RoundStatusRevealed
	round.PhaseEndsAt = nil
	round.CompletedAt = &now
	round.PausedAt = nil

	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("advance round to revealed: %w", err)
	}

	return round, nil
}

func (l roundLifecycle) ResumeRound(ctx context.Context, round domain.Round, now time.Time) (domain.Round, error) {
	if round.PausedAt == nil {
		return round, nil
	}

	if round.PhaseEndsAt != nil {
		shift := now.Sub(*round.PausedAt)
		shifted := round.PhaseEndsAt.Add(shift)
		round.PhaseEndsAt = &shifted
	}

	round.PausedAt = nil
	if err := l.roundRepo.Update(ctx, round); err != nil {
		return domain.Round{}, fmt.Errorf("resume round timing: %w", err)
	}

	return round, nil
}

func presentationOrderAssigned(truthSets []domain.TruthSet) bool {
	for _, truthSet := range truthSets {
		if truthSet.PresentationOrder <= 0 {
			return false
		}
	}

	return true
}

func firstPresentationTruthSet(ctx context.Context, truthSetRepo repository.TruthSetRepository, roundID string) (domain.TruthSet, error) {
	truthSets, err := truthSetRepo.ListByRoundID(ctx, roundID)
	if err != nil {
		return domain.TruthSet{}, fmt.Errorf("list truth sets by round: %w", err)
	}
	if len(truthSets) == 0 {
		return domain.TruthSet{}, domain.ErrActiveTruthSetUnavailable
	}

	first := truthSets[0]
	for _, truthSet := range truthSets[1:] {
		if truthSet.PresentationOrder < first.PresentationOrder {
			first = truthSet
		}
	}

	return first, nil
}

func nextPresentationTruthSet(truthSets []domain.TruthSet, activeTruthSetID string) (domain.TruthSet, bool) {
	if len(truthSets) == 0 {
		return domain.TruthSet{}, false
	}

	currentOrder := 0
	for _, truthSet := range truthSets {
		if truthSet.ID == activeTruthSetID {
			currentOrder = truthSet.PresentationOrder
			break
		}
	}
	if currentOrder == 0 {
		return domain.TruthSet{}, false
	}

	var next domain.TruthSet
	found := false
	for _, truthSet := range truthSets {
		if truthSet.PresentationOrder == currentOrder+1 {
			next = truthSet
			found = true
			break
		}
	}

	return next, found
}

func (l roundLifecycle) newThreeLiesWritingRound(room domain.Room, number int, now time.Time) (domain.Round, error) {
	roundID, err := utils.GenerateID()
	if err != nil {
		return domain.Round{}, fmt.Errorf("generate round id: %w", err)
	}

	phaseEndsAt := now.Add(time.Duration(room.TimePerRound) * time.Second)
	return domain.Round{
		ID:          roundID,
		RoomID:      room.ID,
		RoundNumber: number,
		Status:      domain.RoundStatusWriting,
		StartedAt:   now,
		PhaseEndsAt: &phaseEndsAt,
	}, nil
}
