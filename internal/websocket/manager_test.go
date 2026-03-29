package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/tell-your-story/backend/internal/config"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/internal/service"
)

func TestManagerServeHTTPRejectsInvalidConnection(t *testing.T) {
	fixture := newManagerTestFixture(t)

	testCases := []struct {
		name         string
		roomCode     string
		userID       string
		sessionToken string
		wantStatus   int
	}{
		{
			name:         "missing room code",
			userID:       fixture.hostID,
			sessionToken: fixture.hostSessionToken,
			wantStatus:   400,
		},
		{
			name:         "missing user id",
			roomCode:     fixture.roomCode,
			sessionToken: fixture.hostSessionToken,
			wantStatus:   400,
		},
		{
			name:       "missing session token",
			roomCode:   fixture.roomCode,
			userID:     fixture.hostID,
			wantStatus: 400,
		},
		{
			name:         "unknown room",
			roomCode:     "ZZZZZZ",
			userID:       fixture.hostID,
			sessionToken: fixture.hostSessionToken,
			wantStatus:   404,
		},
		{
			name:         "invalid session token",
			roomCode:     fixture.roomCode,
			userID:       fixture.hostID,
			sessionToken: "invalid-session",
			wantStatus:   401,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wsURL := fixture.websocketURL(tc.roomCode, tc.userID, tc.sessionToken)
			conn, resp, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
			if err == nil {
				_ = conn.Close()
				t.Fatal("Dial() unexpectedly succeeded")
			}

			if resp == nil {
				t.Fatalf("Dial() response = nil, err = %v", err)
			}

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

func TestManagerWebSocketPublishesPresenceLifecycle(t *testing.T) {
	fixture := newManagerTestFixture(t)

	hostConn := fixture.mustDial(t, fixture.roomCode, fixture.hostID, fixture.hostSessionToken)
	defer hostConn.Close()
	fixture.expectInitialHandshake(t, hostConn, fixture.hostID)

	guestConn := fixture.mustDial(t, fixture.roomCode, fixture.guestID, fixture.guestSessionToken)
	fixture.expectInitialHandshake(t, guestConn, fixture.guestID)

	joinedEvent := waitForEventType(t, hostConn, "presence.joined")
	joined := decodePresencePayload(t, joinedEvent)
	if joined.UserID != fixture.guestID {
		t.Fatalf("joined user_id = %q, want %q", joined.UserID, fixture.guestID)
	}

	closeConn(t, guestConn)

	leftEvent := waitForEventType(t, hostConn, "presence.left")
	left := decodePresencePayload(t, leftEvent)
	if left.UserID != fixture.guestID {
		t.Fatalf("left user_id = %q, want %q", left.UserID, fixture.guestID)
	}
}

func TestManagerWebSocketHandlesClientMessages(t *testing.T) {
	fixture := newManagerTestFixture(t)
	fixture.submitStories(t)
	fixture.advanceToVoting(t)
	fixture.submitGuestVote(t)

	hostConn := fixture.mustDial(t, fixture.roomCode, fixture.hostID, fixture.hostSessionToken)
	defer hostConn.Close()
	fixture.expectInitialHandshake(t, hostConn, fixture.hostID)

	if _, err := fixture.roomService.PauseRound(context.Background(), fixture.roomCode, service.RoomActionInput{
		UserID:       fixture.hostID,
		SessionToken: fixture.hostSessionToken,
	}); err != nil {
		t.Fatalf("PauseRound() error = %v", err)
	}

	writeClientMessage(t, hostConn, clientEnvelope{Type: "room.sync"})
	roomStateEvent := waitForEventType(t, hostConn, "room.state")
	roomState := decodeRoomStatePayload(t, roomStateEvent)
	if roomState.Room.Status != domain.RoomStatusPaused {
		t.Fatalf("room status = %q, want %q", roomState.Room.Status, domain.RoomStatusPaused)
	}

	writeClientMessage(t, hostConn, clientEnvelope{Type: "ping"})
	waitForEventType(t, hostConn, "pong")

	writeClientMessage(t, hostConn, clientEnvelope{Type: "story.progress.request"})
	storyProgressEvent := waitForEventType(t, hostConn, "story.progress")
	storyProgress := decodeProgressPayload(t, storyProgressEvent)
	if storyProgress.RoundID != fixture.roundID {
		t.Fatalf("story progress round_id = %q, want %q", storyProgress.RoundID, fixture.roundID)
	}
	if storyProgress.Count != 2 {
		t.Fatalf("story progress count = %d, want 2", storyProgress.Count)
	}

	writeClientMessage(t, hostConn, clientEnvelope{Type: "vote.progress.request"})
	voteProgressEvent := waitForEventType(t, hostConn, "vote.progress")
	voteProgress := decodeProgressPayload(t, voteProgressEvent)
	if voteProgress.RoundID != fixture.roundID {
		t.Fatalf("vote progress round_id = %q, want %q", voteProgress.RoundID, fixture.roundID)
	}
	if voteProgress.Count != 1 {
		t.Fatalf("vote progress count = %d, want 1", voteProgress.Count)
	}

	writeClientMessage(t, hostConn, clientEnvelope{Type: "unsupported"})
	errorEvent := waitForEventType(t, hostConn, "error")
	errorPayload := decodeErrorPayload(t, errorEvent)
	if errorPayload["message"] != "unsupported client message type" {
		t.Fatalf("error message = %q, want unsupported client message type", errorPayload["message"])
	}
}

type managerTestFixture struct {
	server            *httptest.Server
	roomService       *service.RoomService
	storyService      *service.StoryService
	voteService       *service.VoteService
	roomCode          string
	hostID            string
	hostSessionToken  string
	guestID           string
	guestSessionToken string
	roundID           string
	hostStoryID       string
}

func newManagerTestFixture(t *testing.T) *managerTestFixture {
	t.Helper()

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()
	roundRepo := repository.NewInMemoryRoundRepository()
	truthSetRepo := repository.NewInMemoryTruthSetRepository()
	storyRepo := repository.NewInMemoryStoryRepository()
	voteRepo := repository.NewInMemoryVoteRepository()

	roomService := service.NewRoomService(config.GameConfig{
		RoomCodeLength:    6,
		RoomExpiration:    2 * time.Hour,
		MaxPlayersPerRoom: 10,
	}, repository.NewInMemoryGameTypeRepository(), roomRepo, userRepo, roundRepo, truthSetRepo)
	storyService := service.NewStoryService(roomRepo, roundRepo, userRepo, storyRepo, voteRepo)
	voteService := service.NewVoteService(roomRepo, roundRepo, userRepo, storyRepo, voteRepo)

	state, err := roomService.CreateRoom(context.Background(), service.CreateRoomInput{
		HostNickname: "Host",
		MaxRounds:    3,
		TimePerRound: 120,
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	joined, err := roomService.JoinRoom(context.Background(), state.Room.Code, service.JoinRoomInput{
		Nickname: "Guest",
	})
	if err != nil {
		t.Fatalf("JoinRoom() error = %v", err)
	}

	started, err := roomService.StartGame(context.Background(), state.Room.Code, service.RoomActionInput{
		UserID:       state.Room.HostID,
		SessionToken: state.Users[0].SessionToken,
	})
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	var guestID string
	var guestSessionToken string
	for _, user := range joined.Users {
		if !user.IsHost {
			guestID = user.ID
			guestSessionToken = user.SessionToken
			break
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := NewManager(logger, roomService, roomRepo, userRepo, roundRepo, storyRepo, voteRepo)
	server := httptest.NewServer(manager)
	t.Cleanup(server.Close)

	return &managerTestFixture{
		server:            server,
		roomService:       roomService,
		storyService:      storyService,
		voteService:       voteService,
		roomCode:          state.Room.Code,
		hostID:            state.Room.HostID,
		hostSessionToken:  state.Users[0].SessionToken,
		guestID:           guestID,
		guestSessionToken: guestSessionToken,
		roundID:           started.CurrentRound.ID,
	}
}

func (f *managerTestFixture) submitStories(t *testing.T) {
	t.Helper()

	hostStory, err := f.storyService.SubmitStory(context.Background(), service.SubmitStoryInput{
		RoundID:      f.roundID,
		UserID:       f.hostID,
		SessionToken: f.hostSessionToken,
		Title:        "Host story",
		Body:         "A fun fact",
	})
	if err != nil {
		t.Fatalf("SubmitStory(host) error = %v", err)
	}

	if _, err := f.storyService.SubmitStory(context.Background(), service.SubmitStoryInput{
		RoundID:      f.roundID,
		UserID:       f.guestID,
		SessionToken: f.guestSessionToken,
		Title:        "Guest story",
		Body:         "Another fun fact",
	}); err != nil {
		t.Fatalf("SubmitStory(guest) error = %v", err)
	}

	f.hostStoryID = hostStory.ID
}

func (f *managerTestFixture) advanceToVoting(t *testing.T) {
	t.Helper()

	if _, err := f.roomService.NextRound(context.Background(), f.roomCode, service.RoomActionInput{
		UserID:       f.hostID,
		SessionToken: f.hostSessionToken,
	}); err != nil {
		t.Fatalf("NextRound() error = %v", err)
	}
}

func (f *managerTestFixture) submitGuestVote(t *testing.T) {
	t.Helper()

	if _, err := f.voteService.SubmitVote(context.Background(), service.SubmitVoteInput{
		RoundID:      f.roundID,
		UserID:       f.guestID,
		SessionToken: f.guestSessionToken,
		StoryID:      f.hostStoryID,
	}); err != nil {
		t.Fatalf("SubmitVote() error = %v", err)
	}
}

func (f *managerTestFixture) websocketURL(roomCode, userID, sessionToken string) string {
	baseURL := "ws" + strings.TrimPrefix(f.server.URL, "http")
	query := url.Values{}
	if roomCode != "" {
		query.Set("room_code", roomCode)
	}
	if userID != "" {
		query.Set("user_id", userID)
	}
	if sessionToken != "" {
		query.Set("session_token", sessionToken)
	}
	if encoded := query.Encode(); encoded != "" {
		return baseURL + "?" + encoded
	}
	return baseURL
}

func (f *managerTestFixture) mustDial(t *testing.T, roomCode, userID, sessionToken string) *gorillaws.Conn {
	t.Helper()

	conn, resp, err := gorillaws.DefaultDialer.Dial(f.websocketURL(roomCode, userID, sessionToken), nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("Dial() error = %v, status = %d", err, resp.StatusCode)
		}
		t.Fatalf("Dial() error = %v", err)
	}

	return conn
}

func (f *managerTestFixture) expectInitialHandshake(t *testing.T, conn *gorillaws.Conn, wantUserID string) {
	t.Helper()

	roomStateEvent := waitForEventType(t, conn, "room.state")
	roomState := decodeRoomStatePayload(t, roomStateEvent)
	if roomState.Room.Code != f.roomCode {
		t.Fatalf("room code = %q, want %q", roomState.Room.Code, f.roomCode)
	}

	readyEvent := waitForEventType(t, conn, "connection.ready")
	ready := decodePresencePayload(t, readyEvent)
	if ready.UserID != wantUserID {
		t.Fatalf("connection.ready user_id = %q, want %q", ready.UserID, wantUserID)
	}

	joinedEvent := waitForEventType(t, conn, "presence.joined")
	joined := decodePresencePayload(t, joinedEvent)
	if joined.UserID != wantUserID {
		t.Fatalf("presence.joined user_id = %q, want %q", joined.UserID, wantUserID)
	}
}

type testEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func waitForEventType(t *testing.T, conn *gorillaws.Conn, wantType string) testEvent {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	seen := make([]string, 0, 4)

	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline() error = %v", err)
		}

		_, payload, err := conn.ReadMessage()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			t.Fatalf("ReadMessage() error = %v", err)
		}

		var event testEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("json.Unmarshal() event error = %v", err)
		}

		seen = append(seen, event.Type)
		if event.Type == wantType {
			return event
		}
	}

	t.Fatalf("event %q not received, saw %v", wantType, seen)
	return testEvent{}
}

func writeClientMessage(t *testing.T, conn *gorillaws.Conn, message clientEnvelope) {
	t.Helper()

	if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline() error = %v", err)
	}
	if err := conn.WriteJSON(message); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
}

func closeConn(t *testing.T, conn *gorillaws.Conn) {
	t.Helper()

	_ = conn.WriteControl(
		gorillaws.CloseMessage,
		gorillaws.FormatCloseMessage(gorillaws.CloseNormalClosure, "bye"),
		time.Now().Add(time.Second),
	)
	_ = conn.Close()
}

func decodeRoomStatePayload(t *testing.T, event testEvent) service.RoomState {
	t.Helper()

	var payload service.RoomState
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() room state error = %v", err)
	}

	return payload
}

func decodePresencePayload(t *testing.T, event testEvent) presencePayload {
	t.Helper()

	var payload presencePayload
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() presence error = %v", err)
	}

	return payload
}

func decodeProgressPayload(t *testing.T, event testEvent) progressPayload {
	t.Helper()

	var payload progressPayload
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() progress error = %v", err)
	}

	return payload
}

func decodeErrorPayload(t *testing.T, event testEvent) map[string]string {
	t.Helper()

	var payload map[string]string
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error payload error = %v", err)
	}

	return payload
}
