package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tell-your-story/backend/internal/api/respond"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/service"
)

// RoomHandler exposes room endpoints.
type RoomHandler struct {
	roomService *service.RoomService
	notifier    RealtimeNotifier
}

// NewRoomHandler creates a room handler.
func NewRoomHandler(roomService *service.RoomService, notifier RealtimeNotifier) *RoomHandler {
	return &RoomHandler{
		roomService: roomService,
		notifier:    notifier,
	}
}

// CreateRoom handles POST /api/rooms.
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input service.CreateRoomInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	state, err := h.roomService.CreateRoom(r.Context(), input)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, "room created successfully", authenticatedRoomState(state, hostUserFromState(state)))
	h.notifyRoomState(r.Context(), state.Room.Code)
}

// HandleRoomRoutes dispatches room-code routes.
func (h *RoomHandler) HandleRoomRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	path = strings.Trim(path, "/")
	if path == "" {
		respond.Error(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	parts := strings.Split(path, "/")
	code := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		h.getRoom(w, r, code)
		return
	}

	if len(parts) != 2 || r.Method != http.MethodPost {
		respond.Error(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	switch parts[1] {
	case "join":
		h.joinRoom(w, r, code)
	case "leave":
		h.leaveRoom(w, r, code)
	case "start":
		h.startRoom(w, r, code)
	case "pause":
		h.pauseRoom(w, r, code)
	case "next-round":
		h.nextRound(w, r, code)
	default:
		respond.Error(w, http.StatusNotFound, "not_found", "resource not found")
	}
}

func (h *RoomHandler) getRoom(w http.ResponseWriter, r *http.Request, code string) {
	state, err := h.roomService.GetRoomState(r.Context(), code)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, "room fetched successfully", state)
}

func (h *RoomHandler) joinRoom(w http.ResponseWriter, r *http.Request, code string) {
	var input service.JoinRoomInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	state, err := h.roomService.JoinRoom(r.Context(), code, input)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, "user joined room successfully", authenticatedRoomState(state, joinedUserFromState(state)))
	h.notifyRoomState(r.Context(), state.Room.Code)
}

func (h *RoomHandler) leaveRoom(w http.ResponseWriter, r *http.Request, code string) {
	input, err := decodeActionInput(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	state, serviceErr := h.roomService.LeaveRoom(r.Context(), code, input)
	if serviceErr != nil {
		h.writeRoomError(w, serviceErr)
		return
	}

	respond.JSON(w, http.StatusOK, "user left room successfully", state)
	h.notifyRoomState(r.Context(), state.Room.Code)
}

func (h *RoomHandler) startRoom(w http.ResponseWriter, r *http.Request, code string) {
	input, err := decodeActionInput(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	state, serviceErr := h.roomService.StartGame(r.Context(), code, input)
	if serviceErr != nil {
		h.writeRoomError(w, serviceErr)
		return
	}

	respond.JSON(w, http.StatusOK, "room started successfully", state)
	h.notifyRoomState(r.Context(), state.Room.Code)
}

func (h *RoomHandler) pauseRoom(w http.ResponseWriter, r *http.Request, code string) {
	input, err := decodeActionInput(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	state, serviceErr := h.roomService.PauseRound(r.Context(), code, input)
	if serviceErr != nil {
		h.writeRoomError(w, serviceErr)
		return
	}

	respond.JSON(w, http.StatusOK, "room pause state updated successfully", state)
	h.notifyRoomState(r.Context(), state.Room.Code)
}

func (h *RoomHandler) nextRound(w http.ResponseWriter, r *http.Request, code string) {
	input, err := decodeActionInput(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	state, serviceErr := h.roomService.NextRound(r.Context(), code, input)
	if serviceErr != nil {
		h.writeRoomError(w, serviceErr)
		return
	}

	respond.JSON(w, http.StatusOK, "room advanced successfully", state)
	h.notifyRoomState(r.Context(), state.Room.Code)
}

func (h *RoomHandler) writeRoomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrGameTypeNotFound):
		respond.Error(w, http.StatusBadRequest, "invalid_game_type", err.Error())
	case errors.Is(err, domain.ErrRoomNotFound):
		respond.Error(w, http.StatusNotFound, "room_not_found", err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		respond.Error(w, http.StatusNotFound, "user_not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidSessionToken):
		respond.Error(w, http.StatusUnauthorized, "invalid_session", err.Error())
	case errors.Is(err, domain.ErrRoundNotFound):
		respond.Error(w, http.StatusNotFound, "round_not_found", err.Error())
	case errors.Is(err, domain.ErrRoomExpired):
		respond.Error(w, http.StatusGone, "room_expired", err.Error())
	case errors.Is(err, domain.ErrRoomFull):
		respond.Error(w, http.StatusConflict, "room_full", err.Error())
	case errors.Is(err, domain.ErrRoomAlreadyStarted):
		respond.Error(w, http.StatusConflict, "room_already_started", err.Error())
	case errors.Is(err, domain.ErrNotRoomHost):
		respond.Error(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrInvalidRoomState):
		respond.Error(w, http.StatusConflict, "invalid_room_state", err.Error())
	default:
		respond.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	}
}

func decodeActionInput(r *http.Request) (service.RoomActionInput, error) {
	var input service.RoomActionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return service.RoomActionInput{}, errors.New("request body must be valid JSON")
	}

	if strings.TrimSpace(input.UserID) == "" {
		return service.RoomActionInput{}, errors.New("user_id is required")
	}
	if strings.TrimSpace(input.SessionToken) == "" {
		return service.RoomActionInput{}, errors.New("session_token is required")
	}

	return input, nil
}

func authenticatedRoomState(state service.RoomState, user domain.User) service.AuthenticatedRoomState {
	return service.AuthenticatedRoomState{
		RoomState: state,
		Session: service.AuthSession{
			UserID:       user.ID,
			SessionToken: user.SessionToken,
		},
	}
}

func hostUserFromState(state service.RoomState) domain.User {
	for _, user := range state.Users {
		if user.ID == state.Room.HostID {
			return user
		}
	}

	return domain.User{}
}

func joinedUserFromState(state service.RoomState) domain.User {
	if len(state.Users) == 0 {
		return domain.User{}
	}

	return state.Users[len(state.Users)-1]
}

func (h *RoomHandler) notifyRoomState(ctx context.Context, roomCode string) {
	if h.notifier == nil {
		return
	}

	_ = h.notifier.BroadcastRoomState(ctx, roomCode)
}
