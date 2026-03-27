package api

import (
	"log/slog"
	"net/http"
	"strings"

	apidocs "github.com/tell-your-story/backend/internal/api/docs"
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
	notifier handlers.RealtimeNotifier,
	wsHandler http.Handler,
) http.Handler {
	mux := http.NewServeMux()
	roomHandler := handlers.NewRoomHandler(roomService, notifier)
	storyHandler := handlers.NewStoryHandler(storyService, notifier)
	voteHandler := handlers.NewVoteHandler(voteService, notifier)
	docsHandler, err := apidocs.Handler()
	if err != nil {
		logger.Error("failed to initialize api docs", "err", err)
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		respond.JSON(w, http.StatusOK, "service is healthy", map[string]string{
			"status": "ok",
		})
	})
	if docsHandler != nil {
		mux.Handle("/swagger/", http.StripPrefix("/swagger/", docsHandler))
		mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
		})
	}
	if wsHandler != nil {
		mux.Handle("/ws", wsHandler)
	}
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
