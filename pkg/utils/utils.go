package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
)

const (
	roomCodeLetters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	roomCodeDigits  = "23456789"
)

// GenerateID returns a random identifier suitable for in-memory entities.
func GenerateID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}

// GenerateSessionToken returns a random session token suitable for client auth.
func GenerateSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}

// GenerateRoomCode returns a random, user-friendly room code.
func GenerateRoomCode(length int) (string, error) {
	lettersCount, digitsCount := roomCodeParts(length)
	buffer := make([]byte, lettersCount+digitsCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate room code: %w", err)
	}

	var builder strings.Builder
	builder.Grow(len(buffer))

	for index, value := range buffer {
		if index < lettersCount {
			builder.WriteByte(roomCodeLetters[int(value)%len(roomCodeLetters)])
			continue
		}

		builder.WriteByte(roomCodeDigits[int(value)%len(roomCodeDigits)])
	}

	return builder.String(), nil
}

func roomCodeParts(length int) (int, int) {
	if length <= 0 {
		return 4, 2
	}

	if length <= 2 {
		return length - 1, 1
	}

	if length == 6 {
		return 4, 2
	}

	return length - 2, 2
}

// ShuffleStories returns a shuffled copy of the provided stories.
func ShuffleStories(stories []domain.Story) []domain.Story {
	cloned := append([]domain.Story(nil), stories...)
	random := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	random.Shuffle(len(cloned), func(i, j int) {
		cloned[i], cloned[j] = cloned[j], cloned[i]
	})

	return cloned
}

// SanitizeText replaces banned words with "***".
func SanitizeText(input string, bannedWords map[string]struct{}) string {
	if len(bannedWords) == 0 || strings.TrimSpace(input) == "" {
		return input
	}

	tokens := strings.Fields(input)
	for index, token := range tokens {
		trimmed := strings.Trim(token, ".,!?;:'\"()[]{}")
		if trimmed == "" {
			continue
		}

		if _, exists := bannedWords[strings.ToLower(trimmed)]; exists {
			tokens[index] = strings.Replace(token, trimmed, "***", 1)
		}
	}

	return strings.Join(tokens, " ")
}
