package websocket

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/sessionsvc"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
)

var ErrInvalidMessage = errors.New("websocket: invalid message")

// same limit as messagesvc.MaxContentLength (the HTTP send endpoint)
const maxContentLength = 1000

type Hub struct {
	mu       sync.RWMutex
	clients  map[int64]map[*Client]struct{}
	messages *repository.MessageRepository
	groups   *repository.GroupRepository
	sessions *sessionsvc.Service
}

func NewHub(messages *repository.MessageRepository, groups *repository.GroupRepository, sessions *sessionsvc.Service) *Hub {
	return &Hub{
		clients:  make(map[int64]map[*Client]struct{}),
		messages: messages,
		groups:   groups,
		sessions: sessions,
	}
}

// ServeHTTP upgrades GET /ws to a websocket. The Hub is a plain http.Handler, so
// it is routed like any other endpoint. The connection carries the session that
// opened it and the moment that session stops being valid, which is how both a
// logout and an expiry close the socket straight away.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionsvc.CookieName)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	// Parsed before the upgrade, so a token that is not a UUID never opens a
	// connection, and so the connection can be keyed by the 16-byte value instead
	// of by this string, which would otherwise stay alive as long as the socket.
	sessionID, err := uuid.FromString(cookie.Value)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	session, err := h.sessions.Get(cookie.Value)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	connection, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	// Every field is set before the session is told about this connection: the
	// expiry timer may close it from another goroutine the moment it is tracked.
	client := &Client{
		hub:        h,
		connection: connection,
		userID:     session.UserID,
		sessionID:  sessionID,
		expiresAt:  session.ExpiresAt,
		send:       make(chan []byte, 16),
	}
	h.add(client)
	trackClient(sessionID, session.ExpiresAt, client)
	go client.writePump()
	client.readPump()
}

// PublishNotification sends a persisted notification to the user's open pages.
func (h *Hub) PublishNotification(notification *model.Notification) {
	h.publish(notification.UserID, map[string]any{"type": "notification", "notification": notification})
}

func (h *Hub) add(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[*Client]struct{})
	}
	h.clients[client.userID][client] = struct{}{}
}

func (h *Hub) remove(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients := h.clients[client.userID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.userID)
		}
	}
}

func (h *Hub) publish(userID int64, event any) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients[userID] {
		select {
		case client.send <- payload:
		default:
		}
	}
}

type Client struct {
	hub        *Hub
	connection *websocket.Conn
	userID     int64
	sessionID  uuid.UUID
	expiresAt  time.Time
	send       chan []byte
}

type incomingMessage struct {
	Type    string `json:"type"`
	ToUser  *int64 `json:"to_user_id,omitempty"`
	GroupID *int64 `json:"group_id,omitempty"`
	Content string `json:"content"`
	// set only when pictures follow: the sender gets the new message id back
	// under it, then uploads them over HTTP
	ClientID string `json:"client_id,omitempty"`
}

func (c *Client) readPump() {
	defer func() { c.hub.remove(c); untrackClient(c); c.connection.Close() }()
	c.connection.SetReadLimit(64 << 10)
	_ = c.connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.connection.SetPongHandler(func(string) error { return c.connection.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	for {
		var input incomingMessage
		if err := c.connection.ReadJSON(&input); err != nil {
			return
		}
		if incomingMessageHasNullByte(input) {
			c.sendError(ErrInvalidMessage.Error())
			continue
		}
		// The expiry timer normally closes the connection first, but a frame can
		// already be waiting when the session dies, and nothing may be processed
		// on behalf of a session that is no longer valid. Comparing the deadline
		// it was opened with costs nothing, where re-reading the session would be
		// a query on every message.
		if !time.Now().Before(c.expiresAt) {
			c.closeWith(CloseSessionRevoked, "session expired")
			return
		}
		// "someone is writing" is passed on and not stored
		if input.Type == "typing" {
			c.hub.relayTyping(c.userID, input.ToUser, input.GroupID)
			continue
		}
		input.Content = strings.TrimSpace(input.Content)
		if input.Type != "message" || (input.Content == "" && input.ClientID == "") || utf8.RuneCountInString(input.Content) > maxContentLength || len(input.ClientID) > 64 || (input.ToUser == nil) == (input.GroupID == nil) {
			c.sendError(ErrInvalidMessage.Error())
			continue
		}
		allowed, err := c.hub.messages.CanMessage(c.userID, input.ToUser, input.GroupID)
		if err != nil || !allowed {
			c.sendError("message is not permitted")
			continue
		}
		message := &model.Message{FromUserID: c.userID, ToUserID: input.ToUser, GroupID: input.GroupID, Content: input.Content, Images: []string{}}
		if err := c.hub.messages.CreateMessage(message); err != nil {
			c.sendError("could not save message")
			continue
		}
		if input.ClientID != "" {
			payload, _ := json.Marshal(map[string]any{"type": "message_created", "client_id": input.ClientID, "message_id": message.ID})
			select {
			case c.send <- payload:
			default:
			}
			continue
		}
		event := map[string]any{"type": "message", "message": message}
		if input.ToUser != nil {
			c.hub.publish(*input.ToUser, event)
			c.hub.publish(c.userID, event)
		} else if members, err := c.hub.groups.GroupMemberIDs(*input.GroupID); err == nil {
			for _, memberID := range members {
				c.hub.publish(memberID, event)
			}
		}
	}
}

func incomingMessageHasNullByte(input incomingMessage) bool {
	return strings.ContainsRune(input.Type, 0) ||
		strings.ContainsRune(input.Content, 0) ||
		strings.ContainsRune(input.ClientID, 0)
}

func (c *Client) sendError(message string) {
	payload, _ := json.Marshal(map[string]string{"type": "error", "error": message})
	select {
	case c.send <- payload:
	default:
	}
}

// closeWith ends the connection with a status the page can read, rather than
// just dropping it. A bare Close sends no status frame at all, which browsers
// report as 1005 and which is indistinguishable from a lost connection. Safe to
// call from any goroutine: WriteControl may run alongside writePump.
func (c *Client) closeWith(code int, reason string) {
	_ = c.connection.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(5*time.Second),
	)
	_ = c.connection.Close()
}

func (c *Client) writePump() {
	ticker := time.NewTicker(45 * time.Second)
	defer func() { ticker.Stop(); c.connection.Close() }()
	for {
		select {
		case payload, ok := <-c.send:
			_ = c.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok || c.connection.WriteMessage(websocket.TextMessage, payload) != nil {
				return
			}
		case <-ticker.C:
			_ = c.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if c.connection.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}
}
