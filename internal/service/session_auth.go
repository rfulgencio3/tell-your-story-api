package service

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
)

// AuthSession contains the client session credentials for a room user.
type AuthSession struct {
	UserID       string `json:"user_id"`
	SessionToken string `json:"session_token"`
}

// AuthenticatedRoomState extends RoomState with the caller session.
type AuthenticatedRoomState struct {
	RoomState
	Session AuthSession `json:"session"`
}

// AuthenticateUserSession validates the user/session pair and returns the user.
func AuthenticateUserSession(
	ctx context.Context,
	userRepo repository.UserRepository,
	userID string,
	sessionToken string,
) (domain.User, error) {
	user, err := userRepo.GetByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return domain.User{}, domain.ErrInvalidSessionToken
	}

	if subtle.ConstantTimeCompare(
		[]byte(user.SessionToken),
		[]byte(strings.TrimSpace(sessionToken)),
	) != 1 {
		return domain.User{}, domain.ErrInvalidSessionToken
	}

	return user, nil
}
