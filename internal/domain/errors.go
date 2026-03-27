package domain

import "errors"

var (
	// ErrRoomNotFound indicates the requested room does not exist.
	ErrRoomNotFound = errors.New("room not found")
	// ErrRoomExpired indicates the room is no longer active.
	ErrRoomExpired = errors.New("room expired")
	// ErrRoomFull indicates the room reached its participant limit.
	ErrRoomFull = errors.New("room is full")
	// ErrRoomAlreadyStarted indicates the room can no longer accept the operation.
	ErrRoomAlreadyStarted = errors.New("room already started")
	// ErrInvalidRoomState indicates the room is in an invalid state for the operation.
	ErrInvalidRoomState = errors.New("invalid room state")
	// ErrUserNotFound indicates the requested user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrNotRoomHost indicates only the room host can perform the operation.
	ErrNotRoomHost = errors.New("user is not the room host")
	// ErrRoundNotFound indicates the requested round does not exist.
	ErrRoundNotFound = errors.New("round not found")
	// ErrStoryNotFound indicates the requested story does not exist.
	ErrStoryNotFound = errors.New("story not found")
	// ErrStoryAlreadySubmitted indicates a user already submitted a story for the round.
	ErrStoryAlreadySubmitted = errors.New("story already submitted for this round")
	// ErrVoteAlreadyExists indicates a user already voted in the round.
	ErrVoteAlreadyExists = errors.New("vote already submitted for this round")
	// ErrVoteNotFound indicates no vote exists for the requested user and round.
	ErrVoteNotFound = errors.New("vote not found")
	// ErrSelfVote indicates a user attempted to vote for their own story.
	ErrSelfVote = errors.New("users cannot vote for their own story")
)
