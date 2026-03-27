package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/tell-your-story/backend/internal/api/handlers"
	"github.com/tell-your-story/backend/internal/api/middleware"
	"github.com/tell-your-story/backend/internal/api/respond"
	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/service"
)

// NewRouter wires application routes and middleware.
func NewRouter(
	cfg config.Config,
	logger *slog.Logger,
	roomService *service.RoomService,
	storyService *service.StoryService,
	voteService *service.VoteService,
) http.Handler {
	mux := http.NewServeMux()
	roomHandler := handlers.NewRoomHandler(roomService)
	storyHandler := handlers.NewStoryHandler(storyService)
	voteHandler := handlers.NewVoteHandler(voteService)

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		respond.JSON(w, http.StatusOK, "service is healthy", map[string]string{
			"status": "ok",
		})
	})
	mux.HandleFunc("/api/rooms", roomHandler.CreateRoom)
	mux.HandleFunc("/api/rooms/", roomHandler.HandleRoomRoutes)
	mux.HandleFunc("/api/stories", storyHandler.SubmitStory)
	mux.HandleFunc("/api/rounds/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/rounds/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			respond.Error(w, http.StatusNotFound, "not_found", "resource not found")
			return
		}

		roundID := parts[0]
		resource := parts[1]
		if storyHandler.HandleRoundRoutes(w, r, roundID, resource) {
			return
		}

		if voteHandler.HandleRoundRoutes(w, r, roundID, resource) {
			return
		}

		respond.Error(w, http.StatusNotFound, "not_found", "resource not found")
	})
	mux.HandleFunc("/api/votes", voteHandler.SubmitVote)
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if voteHandler.HandleUserRoutes(w, r) {
			return
		}

		respond.Error(w, http.StatusNotFound, "not_found", "resource not found")
	})

	handler := middleware.Logging(logger)(mux)
	handler = middleware.CORS(cfg.CORS.AllowedOrigins)(handler)

	return handler
}
