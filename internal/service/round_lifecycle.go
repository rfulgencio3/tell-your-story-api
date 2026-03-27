package service

import (
	"context"
	"fmt"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

const defaultVotingPhaseDuration = time.Minute

type roundLifecycle struct {
	roomRepo  repository.RoomRepository
	roundRepo repository.RoundRepository
}

func newRoundLifecycle(roomRepo repository.RoomRepository, roundRepo repository.RoundRepository) roundLifecycle {
	return roundLifecycle{
		roomRepo:  roomRepo,
		roundRepo: roundRepo,
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
