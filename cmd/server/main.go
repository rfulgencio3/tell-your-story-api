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
	"github.com/tell-your-story/backend/internal/database"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/internal/service"
	internalws "github.com/tell-your-story/backend/internal/websocket"
	pkglogger "github.com/tell-your-story/backend/pkg/logger"
	"gorm.io/gorm"
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

	roomRepo, userRepo, roundRepo, storyRepo, voteRepo, db, err := buildRepositories(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("failed to initialize storage", "driver", cfg.Storage.Driver, "err", err)
		os.Exit(1)
	}

	if db != nil {
		sqlDB, sqlErr := db.DB()
		if sqlErr != nil {
			logger.Error("failed to resolve sql db", "err", sqlErr)
			os.Exit(1)
		}
		defer func() {
			if closeErr := sqlDB.Close(); closeErr != nil {
				logger.Error("failed to close sql db", "err", closeErr)
			}
		}()
	}

	roomService := service.NewRoomService(cfg.Game, roomRepo, userRepo, roundRepo)
	storyService := service.NewStoryService(roomRepo, roundRepo, userRepo, storyRepo, voteRepo)
	voteService := service.NewVoteService(roomRepo, roundRepo, userRepo, storyRepo, voteRepo)
	wsManager := internalws.NewManager(logger, roomService, roomRepo, userRepo, roundRepo, storyRepo, voteRepo)
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()
	go wsManager.Start(serverCtx)

	router := api.NewRouter(cfg, logger, roomService, storyService, voteService, wsManager, wsManager)

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
	serverCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "err", err)
	}

	logger.Info("server stopped")
}

func buildRepositories(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) (
	repository.RoomRepository,
	repository.UserRepository,
	repository.RoundRepository,
	repository.StoryRepository,
	repository.VoteRepository,
	*gorm.DB,
	error,
) {
	switch cfg.Storage.Driver {
	case "postgres":
		db, err := database.Connect(ctx, cfg.Database)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}

		logger.Info("storage initialized", "driver", cfg.Storage.Driver, "database", cfg.Database.Name)
		return repository.NewGormRoomRepository(db),
			repository.NewGormUserRepository(db),
			repository.NewGormRoundRepository(db),
			repository.NewGormStoryRepository(db),
			repository.NewGormVoteRepository(db),
			db,
			nil
	default:
		logger.Info("storage initialized", "driver", cfg.Storage.Driver)
		return repository.NewInMemoryRoomRepository(),
			repository.NewInMemoryUserRepository(),
			repository.NewInMemoryRoundRepository(),
			repository.NewInMemoryStoryRepository(),
			repository.NewInMemoryVoteRepository(),
			nil,
			nil
	}
}
