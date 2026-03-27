package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/tell-your-story/backend/internal/api"
	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/internal/service"
	pkglogger "github.com/tell-your-story/backend/pkg/logger"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logger := pkglogger.New(cfg.Server.Env)

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()
	roundRepo := repository.NewInMemoryRoundRepository()
	storyRepo := repository.NewInMemoryStoryRepository()
	voteRepo := repository.NewInMemoryVoteRepository()
	roomService := service.NewRoomService(cfg.Game, roomRepo, userRepo, roundRepo)
	storyService := service.NewStoryService(roundRepo, userRepo, storyRepo, voteRepo)
	voteService := service.NewVoteService(roundRepo, userRepo, storyRepo, voteRepo)

	router := api.NewRouter(cfg, logger, roomService, storyService, voteService)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "port", cfg.Server.Port, "environment", cfg.Server.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "err", err)
	}

	logger.Info("server stopped")
}
