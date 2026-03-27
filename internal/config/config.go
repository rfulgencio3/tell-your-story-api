package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort                = "8080"
	defaultEnvironment         = "development"
	defaultRoomCodeLength      = 6
	defaultRoomExpirationHours = 2
	defaultMaxPlayersPerRoom   = 10
)

// Config holds all runtime configuration for the application.
type Config struct {
	Server ServerConfig
	Game   GameConfig
	CORS   CORSConfig
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port string
	Env  string
}

// GameConfig contains gameplay and room settings.
type GameConfig struct {
	RoomCodeLength    int
	RoomExpiration    time.Duration
	MaxPlayersPerRoom int
}

// CORSConfig contains CORS settings.
type CORSConfig struct {
	AllowedOrigins []string
}

// Load reads environment variables and returns a validated config.
func Load() (Config, error) {
	cfg := Config{
		Server: ServerConfig{
			Port: getEnv("PORT", defaultPort),
			Env:  getEnv("ENV", defaultEnvironment),
		},
		Game: GameConfig{
			RoomCodeLength:    getEnvInt("ROOM_CODE_LENGTH", defaultRoomCodeLength),
			RoomExpiration:    time.Duration(getEnvInt("ROOM_EXPIRATION_HOURS", defaultRoomExpirationHours)) * time.Hour,
			MaxPlayersPerRoom: getEnvInt("MAX_PLAYERS_PER_ROOM", defaultMaxPlayersPerRoom),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "*")),
		},
	}

	if cfg.Game.RoomCodeLength < 4 {
		return Config{}, fmt.Errorf("ROOM_CODE_LENGTH must be at least 4")
	}

	if cfg.Game.MaxPlayersPerRoom < 2 {
		return Config{}, fmt.Errorf("MAX_PLAYERS_PER_ROOM must be at least 2")
	}

	if cfg.Game.RoomExpiration <= 0 {
		return Config{}, fmt.Errorf("ROOM_EXPIRATION_HOURS must be positive")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"*"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	if len(origins) == 0 {
		return []string{"*"}
	}

	return origins
}
