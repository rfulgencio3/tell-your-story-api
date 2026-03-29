package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/pkg/utils"
	"github.com/tell-your-story/backend/pkg/validator"
)

const maxRoomCodeGenerationAttempts = 10

// CreateRoomInput contains host and room configuration.
type CreateRoomInput struct {
	HostNickname string `json:"host_nickname"`
	HostAvatar   string `json:"host_avatar_url"`
	GameType     string `json:"game_type"`
	MaxRounds    int    `json:"max_rounds"`
	TimePerRound int    `json:"time_per_round"`
}

// JoinRoomInput contains participant details.
type JoinRoomInput struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar_url"`
}

// RoomActionInput carries the acting user id.
type RoomActionInput struct {
	UserID       string `json:"user_id"`
	SessionToken string `json:"session_token"`
}

// RoomState is the API-facing room snapshot.
type RoomState struct {
	Room         domain.Room     `json:"room"`
	Users        []domain.User   `json:"users"`
	CurrentRound *domain.Round   `json:"current_round,omitempty"`
	ThreeLies    *ThreeLiesState `json:"three_lies,omitempty"`
}

// ThreeLiesState is the API-facing payload for the three-lies-one-truth mode.
type ThreeLiesState struct {
	ActiveTruthSet *domain.TruthSet         `json:"active_truth_set,omitempty"`
	VotingProgress *ThreeLiesVotingProgress `json:"voting_progress,omitempty"`
	Reveal         *ThreeLiesRevealState    `json:"reveal,omitempty"`
	FinalRanking   []ThreeLiesRankingEntry  `json:"final_ranking,omitempty"`
}

// RoomService contains room lifecycle logic.
type RoomService struct {
	cfg              config.GameConfig
	gameTypeRepo     repository.GameTypeRepository
	roomRepo         repository.RoomRepository
	userRepo         repository.UserRepository
	roundRepo        repository.RoundRepository
	truthSetRepo     repository.TruthSetRepository
	truthSetVoteRepo repository.TruthSetVoteRepository
	roomScoreRepo    repository.RoomScoreRepository
	lifecycle        roundLifecycle
}

// NewRoomService creates a RoomService.
func NewRoomService(
	cfg config.GameConfig,
	gameTypeRepo repository.GameTypeRepository,
	roomRepo repository.RoomRepository,
	userRepo repository.UserRepository,
	roundRepo repository.RoundRepository,
	truthSetRepo repository.TruthSetRepository,
	truthSetVoteRepo repository.TruthSetVoteRepository,
	roomScoreRepo repository.RoomScoreRepository,
) *RoomService {
	return &RoomService{
		cfg:              cfg,
		gameTypeRepo:     gameTypeRepo,
		roomRepo:         roomRepo,
		userRepo:         userRepo,
		roundRepo:        roundRepo,
		truthSetRepo:     truthSetRepo,
		truthSetVoteRepo: truthSetVoteRepo,
		roomScoreRepo:    roomScoreRepo,
		lifecycle:        newRoundLifecycle(roomRepo, roundRepo, truthSetRepo, truthSetVoteRepo, roomScoreRepo),
	}
}

// CreateRoom creates a waiting room and its host user.
func (s *RoomService) CreateRoom(ctx context.Context, input CreateRoomInput) (RoomState, error) {
	if err := validator.Nickname(input.HostNickname); err != nil {
		return RoomState{}, err
	}

	if err := validator.RoomSettings(input.MaxRounds, input.TimePerRound); err != nil {
		return RoomState{}, err
	}

	gameType, err := s.resolveGameType(ctx, input.GameType)
	if err != nil {
		return RoomState{}, err
	}

	roomID, err := utils.GenerateID()
	if err != nil {
		return RoomState{}, fmt.Errorf("generate room id: %w", err)
	}

	hostID, err := utils.GenerateID()
	if err != nil {
		return RoomState{}, fmt.Errorf("generate host id: %w", err)
	}

	now := time.Now().UTC()
	roomCode, err := s.generateUniqueRoomCode(ctx)
	if err != nil {
		return RoomState{}, err
	}

	room := domain.Room{
		ID:           roomID,
		Code:         roomCode,
		HostID:       hostID,
		GameTypeID:   gameType.ID,
		GameType:     gameType.Slug,
		MaxRounds:    input.MaxRounds,
		TimePerRound: input.TimePerRound,
		Status:       domain.RoomStatusWaiting,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.cfg.RoomExpiration),
	}

	host := domain.User{
		ID:        hostID,
		RoomID:    roomID,
		Nickname:  strings.TrimSpace(input.HostNickname),
		AvatarURL: strings.TrimSpace(input.HostAvatar),
		IsHost:    true,
		CreatedAt: now,
	}
	host.SessionToken, err = utils.GenerateSessionToken()
	if err != nil {
		return RoomState{}, fmt.Errorf("generate host session token: %w", err)
	}

	if err := s.roomRepo.Create(ctx, room); err != nil {
		return RoomState{}, fmt.Errorf("create room: %w", err)
	}

	if err := s.userRepo.Create(ctx, host); err != nil {
		return RoomState{}, fmt.Errorf("create host user: %w", err)
	}

	return RoomState{
		Room:  room,
		Users: []domain.User{host},
	}, nil
}

// GetRoomState returns the current room snapshot.
func (s *RoomService) GetRoomState(ctx context.Context, code string) (RoomState, error) {
	room, err := s.roomRepo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return RoomState{}, err
	}
	room = s.hydrateRoom(room)

	if err := s.ensureRoomAvailable(ctx, room); err != nil {
		return RoomState{}, err
	}

	room, err = s.syncRoomLifecycle(ctx, room)
	if err != nil {
		return RoomState{}, err
	}

	return s.buildRoomState(ctx, room)
}

// JoinRoom joins a participant to a waiting room.
func (s *RoomService) JoinRoom(ctx context.Context, code string, input JoinRoomInput) (RoomState, error) {
	if err := validator.Nickname(input.Nickname); err != nil {
		return RoomState{}, err
	}

	room, err := s.roomRepo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return RoomState{}, err
	}
	room = s.hydrateRoom(room)

	if err := s.ensureRoomAvailable(ctx, room); err != nil {
		return RoomState{}, err
	}

	if room.Status != domain.RoomStatusWaiting {
		return RoomState{}, domain.ErrRoomAlreadyStarted
	}

	users, err := s.userRepo.ListByRoomID(ctx, room.ID)
	if err != nil {
		return RoomState{}, fmt.Errorf("list room users: %w", err)
	}

	if len(users) >= s.cfg.MaxPlayersPerRoom {
		return RoomState{}, domain.ErrRoomFull
	}

	userID, err := utils.GenerateID()
	if err != nil {
		return RoomState{}, fmt.Errorf("generate user id: %w", err)
	}

	user := domain.User{
		ID:        userID,
		RoomID:    room.ID,
		Nickname:  strings.TrimSpace(input.Nickname),
		AvatarURL: strings.TrimSpace(input.Avatar),
		IsHost:    false,
		CreatedAt: time.Now().UTC(),
	}
	user.SessionToken, err = utils.GenerateSessionToken()
	if err != nil {
		return RoomState{}, fmt.Errorf("generate user session token: %w", err)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return RoomState{}, fmt.Errorf("create user: %w", err)
	}

	return s.buildRoomState(ctx, room)
}

// LeaveRoom removes a user from a room. If the host leaves, the room is finished.
func (s *RoomService) LeaveRoom(ctx context.Context, code string, input RoomActionInput) (RoomState, error) {
	room, err := s.roomRepo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return RoomState{}, err
	}
	room = s.hydrateRoom(room)

	user, err := AuthenticateUserSession(ctx, s.userRepo, input.UserID, input.SessionToken)
	if err != nil {
		return RoomState{}, err
	}

	if user.RoomID != room.ID {
		return RoomState{}, domain.ErrUserNotFound
	}

	if err := s.userRepo.Delete(ctx, user.ID); err != nil {
		return RoomState{}, fmt.Errorf("delete user: %w", err)
	}

	users, err := s.userRepo.ListByRoomID(ctx, room.ID)
	if err != nil {
		return RoomState{}, fmt.Errorf("list room users: %w", err)
	}

	if user.IsHost || len(users) == 0 {
		room.Status = domain.RoomStatusFinished
		if err := s.roomRepo.Update(ctx, room); err != nil {
			return RoomState{}, fmt.Errorf("finish room: %w", err)
		}
	}

	return s.buildRoomState(ctx, room)
}

// StartGame starts the first round or resumes a paused room.
func (s *RoomService) StartGame(ctx context.Context, code string, input RoomActionInput) (RoomState, error) {
	room, err := s.roomRepo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return RoomState{}, err
	}
	room = s.hydrateRoom(room)

	if err := s.ensureRoomHost(ctx, room, strings.TrimSpace(input.UserID), strings.TrimSpace(input.SessionToken)); err != nil {
		return RoomState{}, err
	}

	if err := s.ensureRoomAvailable(ctx, room); err != nil {
		return RoomState{}, err
	}

	switch room.Status {
	case domain.RoomStatusWaiting:
		round, createErr := s.newRound(room, 1)
		if createErr != nil {
			return RoomState{}, createErr
		}
		if err := s.roundRepo.Create(ctx, round); err != nil {
			return RoomState{}, fmt.Errorf("create first round: %w", err)
		}
		room.Status = domain.RoomStatusActive
	case domain.RoomStatusPaused:
		currentRound, roundErr := s.roundRepo.GetCurrentByRoomID(ctx, room.ID)
		if roundErr != nil {
			return RoomState{}, roundErr
		}
		if _, roundErr = s.lifecycle.ResumeRound(ctx, currentRound, time.Now().UTC()); roundErr != nil {
			return RoomState{}, roundErr
		}
		room.Status = domain.RoomStatusActive
	default:
		return RoomState{}, domain.ErrInvalidRoomState
	}

	if err := s.roomRepo.Update(ctx, room); err != nil {
		return RoomState{}, fmt.Errorf("update room status: %w", err)
	}

	return s.buildRoomState(ctx, room)
}

// PauseRound toggles a room between active and paused.
func (s *RoomService) PauseRound(ctx context.Context, code string, input RoomActionInput) (RoomState, error) {
	room, err := s.roomRepo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return RoomState{}, err
	}
	room = s.hydrateRoom(room)

	if err := s.ensureRoomHost(ctx, room, strings.TrimSpace(input.UserID), strings.TrimSpace(input.SessionToken)); err != nil {
		return RoomState{}, err
	}

	switch room.Status {
	case domain.RoomStatusActive:
		room.Status = domain.RoomStatusPaused
	case domain.RoomStatusPaused:
		room.Status = domain.RoomStatusActive
	default:
		return RoomState{}, domain.ErrInvalidRoomState
	}

	if err := s.roomRepo.Update(ctx, room); err != nil {
		return RoomState{}, fmt.Errorf("toggle room pause: %w", err)
	}

	round, err := s.roundRepo.GetCurrentByRoomID(ctx, room.ID)
	if err == nil {
		now := time.Now().UTC()
		if room.Status == domain.RoomStatusPaused {
			round.PausedAt = &now
		} else {
			round, err = s.lifecycle.ResumeRound(ctx, round, now)
			if err != nil {
				return RoomState{}, err
			}
		}
		if room.Status == domain.RoomStatusPaused {
			if updateErr := s.roundRepo.Update(ctx, round); updateErr != nil {
				return RoomState{}, fmt.Errorf("update round pause state: %w", updateErr)
			}
		}
	} else if !errors.Is(err, domain.ErrRoundNotFound) {
		return RoomState{}, err
	}

	room, err = s.syncRoomLifecycle(ctx, room)
	if err != nil {
		return RoomState{}, err
	}

	return s.buildRoomState(ctx, room)
}

// NextRound advances the round phase, starts the next round, or finishes the room.
func (s *RoomService) NextRound(ctx context.Context, code string, input RoomActionInput) (RoomState, error) {
	room, err := s.roomRepo.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return RoomState{}, err
	}
	room = s.hydrateRoom(room)

	if err := s.ensureRoomHost(ctx, room, strings.TrimSpace(input.UserID), strings.TrimSpace(input.SessionToken)); err != nil {
		return RoomState{}, err
	}

	if err := s.ensureRoomAvailable(ctx, room); err != nil {
		return RoomState{}, err
	}

	room, err = s.syncRoomLifecycle(ctx, room)
	if err != nil {
		return RoomState{}, err
	}

	currentRound, err := s.roundRepo.GetCurrentByRoomID(ctx, room.ID)
	if err != nil {
		return RoomState{}, err
	}

	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		return RoomState{}, domain.ErrInvalidRoundState
	}

	now := time.Now().UTC()
	switch currentRound.Status {
	case domain.RoundStatusWriting:
		if _, err := s.lifecycle.advanceToVoting(ctx, currentRound, now); err != nil {
			return RoomState{}, err
		}
	case domain.RoundStatusVoting:
		if _, err := s.lifecycle.advanceToRevealed(ctx, currentRound, now); err != nil {
			return RoomState{}, err
		}
	case domain.RoundStatusRevealed:
		if currentRound.RoundNumber >= room.MaxRounds {
			room.Status = domain.RoomStatusFinished
			if err := s.roomRepo.Update(ctx, room); err != nil {
				return RoomState{}, fmt.Errorf("finish room: %w", err)
			}
			return s.buildRoomState(ctx, room)
		}

		nextRound, err := s.newRound(room, currentRound.RoundNumber+1)
		if err != nil {
			return RoomState{}, err
		}

		room.Status = domain.RoomStatusActive
		if err := s.roomRepo.Update(ctx, room); err != nil {
			return RoomState{}, fmt.Errorf("update room status: %w", err)
		}

		if err := s.roundRepo.Create(ctx, nextRound); err != nil {
			return RoomState{}, fmt.Errorf("create next round: %w", err)
		}
	default:
		return RoomState{}, domain.ErrInvalidRoundState
	}

	return s.buildRoomState(ctx, room)
}

func (s *RoomService) buildRoomState(ctx context.Context, room domain.Room) (RoomState, error) {
	room = s.hydrateRoom(room)

	users, err := s.userRepo.ListByRoomID(ctx, room.ID)
	if err != nil {
		return RoomState{}, fmt.Errorf("list room users: %w", err)
	}

	state := RoomState{
		Room:  room,
		Users: users,
	}

	round, err := s.roundRepo.GetCurrentByRoomID(ctx, room.ID)
	if err == nil {
		room, err = s.syncRoomLifecycle(ctx, room)
		if err != nil {
			return RoomState{}, err
		}
		round, err = s.roundRepo.GetCurrentByRoomID(ctx, room.ID)
		if err != nil {
			return RoomState{}, err
		}
		state.CurrentRound = &round
		if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
			threeLiesState, buildErr := s.buildThreeLiesState(ctx, room, round, users)
			if buildErr != nil {
				return RoomState{}, buildErr
			}
			state.ThreeLies = threeLiesState
		}
	} else if !errors.Is(err, domain.ErrRoundNotFound) {
		return RoomState{}, err
	}

	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) && room.Status == domain.RoomStatusFinished {
		threeLiesState, buildErr := s.buildThreeLiesFinishedState(ctx, room, users)
		if buildErr != nil {
			return RoomState{}, buildErr
		}
		state.ThreeLies = threeLiesState
	}

	return state, nil
}

func (s *RoomService) generateUniqueRoomCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < maxRoomCodeGenerationAttempts; attempt++ {
		code, err := utils.GenerateRoomCode(s.cfg.RoomCodeLength)
		if err != nil {
			return "", err
		}

		_, err = s.roomRepo.GetByCode(ctx, code)
		if errors.Is(err, domain.ErrRoomNotFound) {
			return code, nil
		}

		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("unable to generate a unique room code after %d attempts", maxRoomCodeGenerationAttempts)
}

func (s *RoomService) ensureRoomHost(ctx context.Context, room domain.Room, userID string, sessionToken string) error {
	user, err := AuthenticateUserSession(ctx, s.userRepo, userID, sessionToken)
	if err != nil {
		return err
	}

	if user.RoomID != room.ID || user.ID != room.HostID || !user.IsHost {
		return domain.ErrNotRoomHost
	}

	return nil
}

func (s *RoomService) ensureRoomAvailable(ctx context.Context, room domain.Room) error {
	if time.Now().UTC().After(room.ExpiresAt) {
		room.Status = domain.RoomStatusExpired
		if err := s.roomRepo.Update(ctx, room); err != nil {
			return fmt.Errorf("expire room: %w", err)
		}
		return domain.ErrRoomExpired
	}

	if room.Status == domain.RoomStatusExpired {
		return domain.ErrRoomExpired
	}

	return nil
}

func (s *RoomService) syncRoomLifecycle(ctx context.Context, room domain.Room) (domain.Room, error) {
	room = s.hydrateRoom(room)

	currentRound, err := s.roundRepo.GetCurrentByRoomID(ctx, room.ID)
	if errors.Is(err, domain.ErrRoundNotFound) {
		return room, nil
	}
	if err != nil {
		return domain.Room{}, err
	}

	room, _, err = s.lifecycle.SyncRound(ctx, currentRound)
	if err != nil {
		return domain.Room{}, err
	}

	return room, nil
}

func (s *RoomService) newRound(room domain.Room, number int) (domain.Round, error) {
	roundID, err := utils.GenerateID()
	if err != nil {
		return domain.Round{}, fmt.Errorf("generate round id: %w", err)
	}

	now := time.Now().UTC()
	status := domain.RoundStatusWriting
	phaseDuration := time.Duration(room.TimePerRound) * time.Second
	if domain.IsThreeLiesOneTruthGameTypeID(room.GameTypeID) {
		status = domain.RoundStatusCountdown
		phaseDuration = defaultCountdownPhaseDuration
	}
	phaseEndsAt := now.Add(phaseDuration)

	return domain.Round{
		ID:          roundID,
		RoomID:      room.ID,
		RoundNumber: number,
		Status:      status,
		StartedAt:   now,
		PhaseEndsAt: &phaseEndsAt,
	}, nil
}

func (s *RoomService) resolveGameType(ctx context.Context, requestedSlug string) (domain.GameType, error) {
	return s.gameTypeRepo.GetBySlug(ctx, domain.NormalizeGameTypeSlug(requestedSlug))
}

func (s *RoomService) hydrateRoom(room domain.Room) domain.Room {
	room.GameType = domain.GameTypeSlugFromID(room.GameTypeID)
	return room
}

func (s *RoomService) buildThreeLiesState(ctx context.Context, room domain.Room, round domain.Round, users []domain.User) (*ThreeLiesState, error) {
	state := &ThreeLiesState{}

	if round.ActiveTruthSetID != "" {
		activeTruthSet, err := s.truthSetRepo.GetByID(ctx, round.ActiveTruthSetID)
		if err != nil {
			return nil, err
		}
		state.ActiveTruthSet = &activeTruthSet

		switch round.Status {
		case domain.RoundStatusPresentationVoting:
			votes, err := s.truthSetVoteRepo.ListByTruthSetID(ctx, activeTruthSet.ID)
			if err != nil {
				return nil, err
			}
			eligibleVoters := len(users) - 1
			if eligibleVoters < 0 {
				eligibleVoters = 0
			}
			state.VotingProgress = &ThreeLiesVotingProgress{
				EligibleVoters: eligibleVoters,
				SubmittedVotes: len(votes),
			}
		case domain.RoundStatusReveal, domain.RoundStatusCommentary:
			revealState, err := s.buildThreeLiesRevealState(ctx, activeTruthSet)
			if err != nil {
				return nil, err
			}
			state.Reveal = revealState
		}
	}

	return state, nil
}

func (s *RoomService) buildThreeLiesFinishedState(ctx context.Context, room domain.Room, users []domain.User) (*ThreeLiesState, error) {
	scores, err := s.roomScoreRepo.ListByRoomID(ctx, room.ID)
	if err != nil {
		return nil, err
	}

	return &ThreeLiesState{
		FinalRanking: buildThreeLiesFinalRanking(users, scores),
	}, nil
}

func (s *RoomService) buildThreeLiesRevealState(ctx context.Context, truthSet domain.TruthSet) (*ThreeLiesRevealState, error) {
	votes, err := s.truthSetVoteRepo.ListByTruthSetID(ctx, truthSet.ID)
	if err != nil {
		return nil, err
	}

	_, _, revealedVotes := calculateThreeLiesScoreDelta(truthSet, votes)

	return &ThreeLiesRevealState{
		TruthSet:           truthSet,
		TrueStatementIndex: truthSet.TrueStatementIndex,
		RevealedVotes:      cloneRevealVotes(revealedVotes),
	}, nil
}
