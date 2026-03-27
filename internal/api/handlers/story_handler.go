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

// StoryHandler exposes story endpoints.
type StoryHandler struct {
	storyService *service.StoryService
}

// NewStoryHandler creates a story handler.
func NewStoryHandler(storyService *service.StoryService) *StoryHandler {
	return &StoryHandler{storyService: storyService}
}

// SubmitStory handles POST /api/stories.
func (h *StoryHandler) SubmitStory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var input service.SubmitStoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	story, err := h.storyService.SubmitStory(r.Context(), input)
	if err != nil {
		h.writeStoryError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, "story submitted successfully", story)
}

// HandleRoundRoutes dispatches /api/rounds/{roundId}/stories requests.
func (h *StoryHandler) HandleRoundRoutes(w http.ResponseWriter, r *http.Request, roundID, resource string) bool {
	if resource != "stories" || r.Method != http.MethodGet {
		return false
	}

	stories, err := h.storyService.GetRoundStories(r.Context(), strings.TrimSpace(roundID))
	if err != nil {
		h.writeStoryError(w, err)
		return true
	}

	respond.JSON(w, http.StatusOK, "round stories fetched successfully", stories)
	return true
}

func (h *StoryHandler) writeStoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrRoundNotFound):
		respond.Error(w, http.StatusNotFound, "round_not_found", err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		respond.Error(w, http.StatusNotFound, "user_not_found", err.Error())
	case errors.Is(err, domain.ErrStoryAlreadySubmitted):
		respond.Error(w, http.StatusConflict, "story_already_submitted", err.Error())
	default:
		respond.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	}
}
