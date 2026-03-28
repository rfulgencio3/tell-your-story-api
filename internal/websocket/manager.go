package websocket

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/internal/repository"
	"github.com/tell-your-story/backend/internal/service"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
	pollInterval   = time.Second
)

type eventEnvelope struct {
	Type      string    `json:"type"`
	RoomCode  string    `json:"room_code,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

type clientEnvelope struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type progressPayload struct {
	RoundID string `json:"round_id"`
	Count   int    `json:"count"`
}

type presencePayload struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	IsHost   bool   `json:"is_host"`
}

type client struct {
	conn     *gorillaws.Conn
	manager  *Manager
	roomCode string
	userID   string
	nickname string
	isHost   bool
	send     chan []byte
}

// Manager coordinates websocket clients and room-scoped broadcasts.
type Manager struct {
	logger      *slog.Logger
	roomService *service.RoomService
	roomRepo    repository.RoomRepository
	userRepo    repository.UserRepository
	roundRepo   repository.RoundRepository
	storyRepo   repository.StoryRepository
	voteRepo    repository.VoteRepository
	upgrader    gorillaws.Upgrader

	mu            sync.RWMutex
	clientsByRoom map[string]map[*client]struct{}
	lastRoomState map[string]string
}

// NewManager creates a websocket manager.
func NewManager(
	logger *slog.Logger,
	roomService *service.RoomService,
	roomRepo repository.RoomRepository,
	userRepo repository.UserRepository,
	roundRepo repository.RoundRepository,
	storyRepo repository.StoryRepository,
	voteRepo repository.VoteRepository,
) *Manager {
	return &Manager{
		logger:      logger,
		roomService: roomService,
		roomRepo:    roomRepo,
		userRepo:    userRepo,
		roundRepo:   roundRepo,
		storyRepo:   storyRepo,
		voteRepo:    voteRepo,
		upgrader: gorillaws.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		clientsByRoom: make(map[string]map[*client]struct{}),
		lastRoomState: make(map[string]string),
	}
}

// Start begins periodic synchronization for subscribed rooms.
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.syncActiveRooms(ctx)
		}
	}
}

// ServeHTTP upgrades the request and subscribes the client to a room.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	roomCode := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("room_code")))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	sessionToken := strings.TrimSpace(r.URL.Query().Get("session_token"))
	if roomCode == "" {
		m.logger.Warn("websocket rejected", "reason", "missing_room_code", "origin", r.Header.Get("Origin"))
		http.Error(w, "room_code is required", http.StatusBadRequest)
		return
	}
	if userID == "" {
		m.logger.Warn("websocket rejected", "reason", "missing_user_id", "room_code", roomCode, "origin", r.Header.Get("Origin"))
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if sessionToken == "" {
		m.logger.Warn("websocket rejected", "reason", "missing_session_token", "room_code", roomCode, "user_id", userID, "origin", r.Header.Get("Origin"))
		http.Error(w, "session_token is required", http.StatusBadRequest)
		return
	}

	state, err := m.roomService.GetRoomState(r.Context(), roomCode)
	if err != nil {
		m.logger.Warn("websocket rejected", "reason", "room_state_unavailable", "room_code", roomCode, "user_id", userID, "origin", r.Header.Get("Origin"), "err", err)
		http.Error(w, err.Error(), httpStatusFromDomainError(err))
		return
	}

	user, err := m.userRepo.GetByID(r.Context(), userID)
	if err != nil || subtle.ConstantTimeCompare([]byte(user.SessionToken), []byte(sessionToken)) != 1 {
		m.logger.Warn("websocket rejected", "reason", "invalid_session", "room_code", roomCode, "user_id", userID, "origin", r.Header.Get("Origin"), "user_lookup_error", err != nil)
		http.Error(w, domain.ErrInvalidSessionToken.Error(), http.StatusUnauthorized)
		return
	}
	if user.RoomID != state.Room.ID {
		m.logger.Warn("websocket rejected", "reason", "user_not_in_room", "room_code", roomCode, "user_id", userID, "origin", r.Header.Get("Origin"), "user_room_id", user.RoomID, "state_room_id", state.Room.ID)
		http.Error(w, "user is not part of the room", http.StatusForbidden)
		return
	}

	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.logger.Error("failed to upgrade websocket connection", "room_code", roomCode, "user_id", userID, "origin", r.Header.Get("Origin"), "err", err)
		return
	}

	m.logger.Info("websocket connected", "room_code", roomCode, "user_id", userID, "origin", r.Header.Get("Origin"))

	client := &client{
		conn:     conn,
		manager:  m,
		roomCode: roomCode,
		userID:   user.ID,
		nickname: user.Nickname,
		isHost:   user.IsHost,
		send:     make(chan []byte, 8),
	}

	m.register(client)
	if err := m.sendRoomStateToClient(client, state); err != nil {
		m.logger.Warn("failed to send initial room state", "room_code", roomCode, "err", err)
		m.unregister(client)
		_ = conn.Close()
		return
	}
	_ = m.sendEventToClient(client, eventEnvelope{
		Type:      "connection.ready",
		RoomCode:  roomCode,
		Timestamp: time.Now().UTC(),
		Data: presencePayload{
			UserID:   client.userID,
			Nickname: client.nickname,
			IsHost:   client.isHost,
		},
	})
	_ = m.broadcastEvent(roomCode, eventEnvelope{
		Type:      "presence.joined",
		RoomCode:  roomCode,
		Timestamp: time.Now().UTC(),
		Data: presencePayload{
			UserID:   client.userID,
			Nickname: client.nickname,
			IsHost:   client.isHost,
		},
	})

	go client.writePump()
	go client.readPump()
}

// BroadcastRoomState broadcasts the latest room snapshot to subscribers.
func (m *Manager) BroadcastRoomState(ctx context.Context, roomCode string) error {
	return m.broadcastRoomState(ctx, strings.ToUpper(strings.TrimSpace(roomCode)), true)
}

// BroadcastStoryProgress broadcasts story submission progress for the round.
func (m *Manager) BroadcastStoryProgress(ctx context.Context, roundID string) error {
	roomCode, err := m.roomCodeFromRoundID(ctx, roundID)
	if err != nil {
		return err
	}

	stories, err := m.storyRepo.ListByRoundID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return fmt.Errorf("list stories for websocket event: %w", err)
	}

	return m.broadcastEvent(roomCode, eventEnvelope{
		Type:      "story.progress",
		RoomCode:  roomCode,
		Timestamp: time.Now().UTC(),
		Data: progressPayload{
			RoundID: strings.TrimSpace(roundID),
			Count:   len(stories),
		},
	})
}

// BroadcastVoteProgress broadcasts vote progress for the round.
func (m *Manager) BroadcastVoteProgress(ctx context.Context, roundID string) error {
	roomCode, err := m.roomCodeFromRoundID(ctx, roundID)
	if err != nil {
		return err
	}

	votes, err := m.voteRepo.ListByRoundID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return fmt.Errorf("list votes for websocket event: %w", err)
	}

	return m.broadcastEvent(roomCode, eventEnvelope{
		Type:      "vote.progress",
		RoomCode:  roomCode,
		Timestamp: time.Now().UTC(),
		Data: progressPayload{
			RoundID: strings.TrimSpace(roundID),
			Count:   len(votes),
		},
	})
}

// BroadcastTopStory broadcasts the reveal payload for the round.
func (m *Manager) BroadcastTopStory(ctx context.Context, roundID string, payload any) error {
	roomCode, err := m.roomCodeFromRoundID(ctx, roundID)
	if err != nil {
		return err
	}

	return m.broadcastEvent(roomCode, eventEnvelope{
		Type:      "round.revealed",
		RoomCode:  roomCode,
		Timestamp: time.Now().UTC(),
		Data:      payload,
	})
}

func (m *Manager) register(wsClient *client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clientsByRoom[wsClient.roomCode]; !exists {
		m.clientsByRoom[wsClient.roomCode] = make(map[*client]struct{})
	}
	m.clientsByRoom[wsClient.roomCode][wsClient] = struct{}{}
}

func (m *Manager) unregister(wsClient *client) {
	m.mu.Lock()

	roomClients := m.clientsByRoom[wsClient.roomCode]
	if roomClients == nil {
		m.mu.Unlock()
		return
	}

	if _, exists := roomClients[wsClient]; !exists {
		m.mu.Unlock()
		return
	}

	delete(roomClients, wsClient)
	close(wsClient.send)

	if len(roomClients) == 0 {
		delete(m.clientsByRoom, wsClient.roomCode)
		delete(m.lastRoomState, wsClient.roomCode)
	}

	m.mu.Unlock()

	_ = m.broadcastEvent(wsClient.roomCode, eventEnvelope{
		Type:      "presence.left",
		RoomCode:  wsClient.roomCode,
		Timestamp: time.Now().UTC(),
		Data: presencePayload{
			UserID:   wsClient.userID,
			Nickname: wsClient.nickname,
			IsHost:   wsClient.isHost,
		},
	})
}

func (m *Manager) syncActiveRooms(ctx context.Context) {
	for _, roomCode := range m.activeRoomCodes() {
		if err := m.broadcastRoomState(ctx, roomCode, false); err != nil && !errors.Is(err, domain.ErrRoomNotFound) {
			m.logger.Debug("websocket room sync skipped", "room_code", roomCode, "err", err)
		}
	}
}

func (m *Manager) activeRoomCodes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roomCodes := make([]string, 0, len(m.clientsByRoom))
	for roomCode := range m.clientsByRoom {
		roomCodes = append(roomCodes, roomCode)
	}

	return roomCodes
}

func (m *Manager) broadcastRoomState(ctx context.Context, roomCode string, force bool) error {
	state, err := m.roomService.GetRoomState(ctx, roomCode)
	if err != nil {
		if errors.Is(err, domain.ErrRoomExpired) {
			return m.broadcastEvent(roomCode, eventEnvelope{
				Type:      "room.expired",
				RoomCode:  roomCode,
				Timestamp: time.Now().UTC(),
			})
		}
		return err
	}

	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal room state payload: %w", err)
	}

	if !force && !m.shouldBroadcastRoomState(roomCode, string(payload)) {
		return nil
	}

	m.setLastRoomState(roomCode, string(payload))

	return m.broadcastEvent(roomCode, eventEnvelope{
		Type:      "room.state",
		RoomCode:  roomCode,
		Timestamp: time.Now().UTC(),
		Data:      state,
	})
}

func (m *Manager) sendRoomStateToClient(client *client, state service.RoomState) error {
	envelope := eventEnvelope{
		Type:      "room.state",
		RoomCode:  client.roomCode,
		Timestamp: time.Now().UTC(),
		Data:      state,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal initial room state: %w", err)
	}

	select {
	case client.send <- payload:
	default:
		return fmt.Errorf("client send buffer full")
	}

	statePayload, err := json.Marshal(state)
	if err == nil {
		m.setLastRoomState(client.roomCode, string(statePayload))
	}

	return nil
}

func (m *Manager) sendEventToClient(client *client, event eventEnvelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal direct websocket event: %w", err)
	}

	select {
	case client.send <- payload:
	default:
		return fmt.Errorf("client send buffer full")
	}

	return nil
}

func (m *Manager) shouldBroadcastRoomState(roomCode, payload string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lastPayload, exists := m.lastRoomState[roomCode]
	return !exists || lastPayload != payload
}

func (m *Manager) setLastRoomState(roomCode, payload string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastRoomState[roomCode] = payload
}

func (m *Manager) broadcastEvent(roomCode string, event eventEnvelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal websocket event: %w", err)
	}

	m.mu.RLock()
	clients := m.clientsByRoom[roomCode]
	clientList := make([]*client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	m.mu.RUnlock()

	for _, client := range clientList {
		select {
		case client.send <- payload:
		default:
			m.unregister(client)
		}
	}

	return nil
}

func (m *Manager) roomCodeFromRoundID(ctx context.Context, roundID string) (string, error) {
	round, err := m.roundRepo.GetByID(ctx, strings.TrimSpace(roundID))
	if err != nil {
		return "", err
	}

	room, err := m.roomRepo.GetByID(ctx, round.RoomID)
	if err != nil {
		return "", err
	}

	return room.Code, nil
}

func httpStatusFromDomainError(err error) int {
	switch {
	case errors.Is(err, domain.ErrRoomNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrRoomExpired):
		return http.StatusGone
	default:
		return http.StatusBadRequest
	}
}

func (c *client) readPump() {
	defer func() {
		c.manager.unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if gorillaws.IsUnexpectedCloseError(err, gorillaws.CloseGoingAway, gorillaws.CloseAbnormalClosure) {
				c.manager.logger.Debug("websocket read closed", "room_code", c.roomCode, "err", err)
			}
			break
		}

		if err := c.handleMessage(payload); err != nil {
			_ = c.manager.sendEventToClient(c, eventEnvelope{
				Type:      "error",
				RoomCode:  c.roomCode,
				Timestamp: time.Now().UTC(),
				Data: map[string]string{
					"message": err.Error(),
				},
			})
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(gorillaws.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(gorillaws.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gorillaws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) handleMessage(payload []byte) error {
	var message clientEnvelope
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("invalid client message")
	}

	switch strings.TrimSpace(message.Type) {
	case "ping":
		return c.manager.sendEventToClient(c, eventEnvelope{
			Type:      "pong",
			RoomCode:  c.roomCode,
			Timestamp: time.Now().UTC(),
		})
	case "room.sync":
		state, err := c.manager.roomService.GetRoomState(context.Background(), c.roomCode)
		if err != nil {
			return err
		}
		return c.manager.sendRoomStateToClient(c, state)
	case "story.progress.request":
		return c.sendCurrentStoryProgress()
	case "vote.progress.request":
		return c.sendCurrentVoteProgress()
	default:
		return fmt.Errorf("unsupported client message type")
	}
}

func (c *client) sendCurrentStoryProgress() error {
	state, err := c.manager.roomService.GetRoomState(context.Background(), c.roomCode)
	if err != nil {
		return err
	}
	if state.CurrentRound == nil {
		return fmt.Errorf("room has no active round")
	}

	stories, err := c.manager.storyRepo.ListByRoundID(context.Background(), state.CurrentRound.ID)
	if err != nil {
		return err
	}

	return c.manager.sendEventToClient(c, eventEnvelope{
		Type:      "story.progress",
		RoomCode:  c.roomCode,
		Timestamp: time.Now().UTC(),
		Data: progressPayload{
			RoundID: state.CurrentRound.ID,
			Count:   len(stories),
		},
	})
}

func (c *client) sendCurrentVoteProgress() error {
	state, err := c.manager.roomService.GetRoomState(context.Background(), c.roomCode)
	if err != nil {
		return err
	}
	if state.CurrentRound == nil {
		return fmt.Errorf("room has no active round")
	}

	votes, err := c.manager.voteRepo.ListByRoundID(context.Background(), state.CurrentRound.ID)
	if err != nil {
		return err
	}

	return c.manager.sendEventToClient(c, eventEnvelope{
		Type:      "vote.progress",
		RoomCode:  c.roomCode,
		Timestamp: time.Now().UTC(),
		Data: progressPayload{
			RoundID: state.CurrentRound.ID,
			Count:   len(votes),
		},
	})
}
