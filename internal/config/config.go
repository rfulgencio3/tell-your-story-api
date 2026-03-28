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
	defaultStorageDriver       = "memory"
	defaultRoomCodeLength      = 6
	defaultRoomExpirationHours = 2
	defaultMaxPlayersPerRoom   = 10
	defaultDBHost              = "localhost"
	defaultDBPort              = 5432
	defaultDBUser              = "postgres"
	defaultDBPassword          = "postgres"
	defaultDBName              = "tell_your_story"
	defaultDBSSLMode           = "disable"
)

// Config holds all runtime configuration for the application.
type Config struct {
	Server   ServerConfig
	Storage  StorageConfig
	Database DatabaseConfig
	Game     GameConfig
	CORS     CORSConfig
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port string
	Env  string
}

// StorageConfig contains repository backend selection settings.
type StorageConfig struct {
	Driver string
}

// DatabaseConfig contains PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
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
		Storage: StorageConfig{
			Driver: strings.ToLower(getEnv("STORAGE_DRIVER", defaultStorageDriver)),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", defaultDBHost),
			Port:     getEnvInt("DB_PORT", defaultDBPort),
			User:     getEnv("DB_USER", defaultDBUser),
			Password: getEnv("DB_PASSWORD", defaultDBPassword),
			Name:     getEnv("DB_NAME", defaultDBName),
			SSLMode:  getEnv("DB_SSLMODE", defaultDBSSLMode),
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

	if cfg.Storage.Driver != "memory" && cfg.Storage.Driver != "postgres" {
		return Config{}, fmt.Errorf("STORAGE_DRIVER must be one of: memory, postgres")
	}

	if cfg.Database.Port <= 0 {
		return Config{}, fmt.Errorf("DB_PORT must be positive")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := normalizeEnvValue(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	raw := normalizeEnvValue(os.Getenv(key))
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
	raw = normalizeEnvValue(raw)
	if raw == "" {
		return []string{"*"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := normalizeEnvValue(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	if len(origins) == 0 {
		return []string{"*"}
	}

	return origins
}

func normalizeEnvValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') || (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			trimmed = trimmed[1 : len(trimmed)-1]
		}
	}

	return strings.TrimSpace(trimmed)
}
