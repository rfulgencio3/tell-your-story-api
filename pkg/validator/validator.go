package validator

import (
	"errors"
	"fmt"
	"strings"
)

const (
	minRounds          = 1
	maxRounds          = 5
	minTimePerRound    = 60
	maxTimePerRound    = 300
	maxNicknameLength  = 50
	maxTitleLength     = 100
	maxStoryBodyLength = 500
)

// RoomSettings validates host-selected room settings.
func RoomSettings(maxRoundsValue, timePerRound int) error {
	if maxRoundsValue < minRounds || maxRoundsValue > maxRounds {
		return fmt.Errorf("max_rounds must be between %d and %d", minRounds, maxRounds)
	}

	if timePerRound < minTimePerRound || timePerRound > maxTimePerRound {
		return fmt.Errorf("time_per_round must be between %d and %d seconds", minTimePerRound, maxTimePerRound)
	}

	return nil
}

// Nickname validates participant nicknames.
func Nickname(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("nickname is required")
	}

	if len([]rune(trimmed)) > maxNicknameLength {
		return fmt.Errorf("nickname must be at most %d characters", maxNicknameLength)
	}

	return nil
}

// Story validates story title and body constraints.
func Story(title, body string) error {
	trimmedTitle := strings.TrimSpace(title)
	trimmedBody := strings.TrimSpace(body)

	if trimmedTitle == "" {
		return errors.New("title is required")
	}

	if trimmedBody == "" {
		return errors.New("body is required")
	}

	if len([]rune(trimmedTitle)) > maxTitleLength {
		return fmt.Errorf("title must be at most %d characters", maxTitleLength)
	}

	if len([]rune(trimmedBody)) > maxStoryBodyLength {
		return fmt.Errorf("body must be at most %d characters", maxStoryBodyLength)
	}

	return nil
}
