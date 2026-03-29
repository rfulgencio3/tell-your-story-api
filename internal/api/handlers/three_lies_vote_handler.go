package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tell-your-story/backend/internal/api/respond"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/internal/service"
)

// ThreeLiesVoteHandler exposes voting endpoints for three-lies-one-truth.
type ThreeLiesVoteHandler struct {
	voteService *service.ThreeLiesVoteService
	roomRepo    repository.RoomRepository
	notifier    RealtimeNotifier
}

// NewThreeLiesVoteHandler creates a three-lies vote handler.
func NewThreeLiesVoteHandler(
	voteService *service.ThreeLiesVoteService,
	roomRepo repository.RoomRepository,
	notifier RealtimeNotifier,
) *ThreeLiesVoteHandler {
	return &ThreeLiesVoteHandler{
		voteService: voteService,
		roomRepo:    roomRepo,
		notifier:    notifier,
	}
}

// SubmitVote handles POST /api/three-lies/votes.
func (h *ThreeLiesVoteHandler) SubmitVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input service.SubmitTruthSetVoteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.SessionToken) == "" {
		respond.Error(w, http.StatusBadRequest, "bad_request", "user_id and session_token are required")
		return
	}

	vote, created, err := h.voteService.SubmitVote(r.Context(), input)
	if err != nil {
		h.writeVoteError(w, err)
		return
	}

	status := http.StatusOK
	message := "truth set vote updated successfully"
	if created {
		status = http.StatusCreated
		message = "truth set vote submitted successfully"
	}

	respond.JSON(w, status, message, vote)
	if h.notifier != nil {
		room, roomErr := h.roomRepo.GetByID(r.Context(), vote.RoomID)
		if roomErr == nil {
			_ = h.notifier.BroadcastRoomState(r.Context(), room.Code)
		}
	}
}

func (h *ThreeLiesVoteHandler) writeVoteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrRoundNotFound):
		respond.Error(w, http.StatusNotFound, "round_not_found", err.Error())
	case errors.Is(err, domain.ErrTruthSetNotFound):
		respond.Error(w, http.StatusNotFound, "truth_set_not_found", err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		respond.Error(w, http.StatusNotFound, "user_not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidSessionToken):
		respond.Error(w, http.StatusUnauthorized, "invalid_session", err.Error())
	case errors.Is(err, domain.ErrRoomExpired):
		respond.Error(w, http.StatusGone, "room_expired", err.Error())
	case errors.Is(err, domain.ErrSelfVote):
		respond.Error(w, http.StatusConflict, "self_vote_not_allowed", err.Error())
	case errors.Is(err, domain.ErrInvalidRoomState):
		respond.Error(w, http.StatusConflict, "invalid_room_state", err.Error())
	case errors.Is(err, domain.ErrInvalidRoundState):
		respond.Error(w, http.StatusConflict, "invalid_round_state", err.Error())
	case errors.Is(err, domain.ErrActiveTruthSetUnavailable):
		respond.Error(w, http.StatusConflict, "active_truth_set_unavailable", err.Error())
	default:
		respond.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	}
}
