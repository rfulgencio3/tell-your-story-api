package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tell-your-story/backend/internal/api/respond"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/service"
)

// VoteHandler exposes vote endpoints.
type VoteHandler struct {
	voteService *service.VoteService
	notifier    RealtimeNotifier
}

// NewVoteHandler creates a vote handler.
func NewVoteHandler(voteService *service.VoteService, notifier RealtimeNotifier) *VoteHandler {
	return &VoteHandler{
		voteService: voteService,
		notifier:    notifier,
	}
}

// SubmitVote handles POST /api/votes.
func (h *VoteHandler) SubmitVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input service.SubmitVoteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.SessionToken) == "" {
		respond.Error(w, http.StatusBadRequest, "bad_request", "user_id and session_token are required")
		return
	}

	vote, err := h.voteService.SubmitVote(r.Context(), input)
	if err != nil {
		h.writeVoteError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, "vote submitted successfully", vote)
	if h.notifier != nil {
		_ = h.notifier.BroadcastVoteProgress(r.Context(), vote.RoundID)
	}
}

// HandleRoundRoutes dispatches vote-related round endpoints.
func (h *VoteHandler) HandleRoundRoutes(w http.ResponseWriter, r *http.Request, roundID, resource string) bool {
	switch {
	case resource == "votes" && r.Method == http.MethodGet:
		votes, err := h.voteService.GetRoundVotes(r.Context(), strings.TrimSpace(roundID))
		if err != nil {
			h.writeVoteError(w, err)
			return true
		}

		respond.JSON(w, http.StatusOK, "round votes fetched successfully", votes)
		return true
	case resource == "top-story" && r.Method == http.MethodGet:
		topStory, err := h.voteService.GetTopStory(r.Context(), strings.TrimSpace(roundID))
		if err != nil {
			h.writeVoteError(w, err)
			return true
		}

		respond.JSON(w, http.StatusOK, "top story fetched successfully", topStory)
		if h.notifier != nil {
			_ = h.notifier.BroadcastTopStory(r.Context(), strings.TrimSpace(roundID), topStory)
		}
		return true
	default:
		return false
	}
}

// HandleUserRoutes dispatches GET /api/users/{userId}/rounds/{roundId}/vote.
func (h *VoteHandler) HandleUserRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/users/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "rounds" || parts[3] != "vote" {
		return false
	}

	sessionToken := strings.TrimSpace(r.URL.Query().Get("session_token"))
	if sessionToken == "" {
		respond.Error(w, http.StatusBadRequest, "bad_request", "session_token is required")
		return true
	}

	userVote, err := h.voteService.GetUserVote(r.Context(), parts[0], parts[2], sessionToken)
	if err != nil {
		h.writeVoteError(w, err)
		return true
	}

	respond.JSON(w, http.StatusOK, "user vote fetched successfully", userVote)
	return true
}

func (h *VoteHandler) writeVoteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrRoundNotFound):
		respond.Error(w, http.StatusNotFound, "round_not_found", err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		respond.Error(w, http.StatusNotFound, "user_not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidSessionToken):
		respond.Error(w, http.StatusUnauthorized, "invalid_session", err.Error())
	case errors.Is(err, domain.ErrRoomExpired):
		respond.Error(w, http.StatusGone, "room_expired", err.Error())
	case errors.Is(err, domain.ErrStoryNotFound):
		respond.Error(w, http.StatusNotFound, "story_not_found", err.Error())
	case errors.Is(err, domain.ErrVoteNotFound):
		respond.Error(w, http.StatusNotFound, "vote_not_found", err.Error())
	case errors.Is(err, domain.ErrVoteAlreadyExists):
		respond.Error(w, http.StatusConflict, "vote_already_submitted", err.Error())
	case errors.Is(err, domain.ErrSelfVote):
		respond.Error(w, http.StatusConflict, "self_vote_not_allowed", err.Error())
	case errors.Is(err, domain.ErrInvalidRoomState):
		respond.Error(w, http.StatusConflict, "invalid_round_state", err.Error())
	default:
		respond.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	}
}
