package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

const defaultVotingPhaseDuration = time.Minute
const defaultCountdownPhaseDuration = 3 * time.Second

type roundLifecycle struct {
	roomRepo     repository.RoomRepository
	roundRepo    repository.RoundRepository
	truthSetRepo repository.TruthSetRepository
}

func newRoundLifecycle(
	roomRepo repository.RoomRepository,
	roundRepo repository.RoundRepository,
	truthSetRepo repository.TruthSetRepository,
) roundLifecycle {
	return roundLifecycle{
		roomRepo:     roomRepo,
		roundRepo:    roundRepo,
		truthSetRepo: truthSetRepo,
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
