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

const roomCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateID returns a random identifier suitable for in-memory entities.
func GenerateID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}

// GenerateRoomCode returns a random, user-friendly room code.
func GenerateRoomCode(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate room code: %w", err)
	}

	var builder strings.Builder
	builder.Grow(length)

	for _, value := range buffer {
		builder.WriteByte(roomCodeAlphabet[int(value)%len(roomCodeAlphabet)])
	}

	return builder.String(), nil
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
