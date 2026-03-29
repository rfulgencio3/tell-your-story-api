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

// ThreeLiesTruthSetHandler exposes writing endpoints for three-lies-one-truth.
type ThreeLiesTruthSetHandler struct {
	truthSetService *service.TruthSetService
	roomRepo        repository.RoomRepository
	notifier        RealtimeNotifier
}

// NewThreeLiesTruthSetHandler creates a three-lies truth set handler.
func NewThreeLiesTruthSetHandler(
	truthSetService *service.TruthSetService,
	roomRepo repository.RoomRepository,
	notifier RealtimeNotifier,
) *ThreeLiesTruthSetHandler {
	return &ThreeLiesTruthSetHandler{
		truthSetService: truthSetService,
		roomRepo:        roomRepo,
		notifier:        notifier,
	}
}

// SubmitTruthSet handles POST /api/three-lies/truth-sets.
func (h *ThreeLiesTruthSetHandler) SubmitTruthSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input service.SubmitTruthSetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.SessionToken) == "" {
		respond.Error(w, http.StatusBadRequest, "bad_request", "user_id and session_token are required")
		return
	}

	truthSet, created, err := h.truthSetService.SubmitTruthSet(r.Context(), input)
	if err != nil {
		h.writeTruthSetError(w, err)
		return
	}

	status := http.StatusOK
	message := "truth set updated successfully"
	if created {
		status = http.StatusCreated
		message = "truth set submitted successfully"
	}

	respond.JSON(w, status, message, truthSet)
	if h.notifier != nil {
		room, roomErr := h.roomRepo.GetByID(r.Context(), truthSet.RoomID)
		if roomErr == nil {
			_ = h.notifier.BroadcastRoomState(r.Context(), room.Code)
		}
	}
}

func (h *ThreeLiesTruthSetHandler) writeTruthSetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrRoundNotFound):
		respond.Error(w, http.StatusNotFound, "round_not_found", err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		respond.Error(w, http.StatusNotFound, "user_not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidSessionToken):
		respond.Error(w, http.StatusUnauthorized, "invalid_session", err.Error())
	case errors.Is(err, domain.ErrRoomExpired):
		respond.Error(w, http.StatusGone, "room_expired", err.Error())
	case errors.Is(err, domain.ErrInvalidRoomState):
		respond.Error(w, http.StatusConflict, "invalid_room_state", err.Error())
	case errors.Is(err, domain.ErrInvalidRoundState):
		respond.Error(w, http.StatusConflict, "invalid_round_state", err.Error())
	default:
		respond.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	}
}
