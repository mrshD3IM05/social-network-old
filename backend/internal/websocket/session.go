package websocket

import (
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

// CloseSessionRevoked is the status sent to a connection whose session is no
// longer valid, because it was logged out or has expired. 4401 sits in the
// 4000-4999 range the WebSocket spec reserves for applications, so the page can
// tell it apart from the 1005 a browser reports for a connection that was
// closed without a status at all.
const CloseSessionRevoked = 4401

// liveSession is the in-memory view of one database session: when it expires,
// and every connection opened with its cookie. A session outlives any single
// connection, because the same cookie is used by every tab and device, so the
// expiry timer belongs to the session rather than to a connection.
type liveSession struct {
	expiresAt time.Time
	clients   map[*Client]struct{}
	timer     *time.Timer
}

// liveSessions tracks the sessions that currently hold at least one open
// connection. The database stays the source of truth for whether a session is
// valid; this only exists so that validity can be acted on the moment it ends,
// instead of at the next request. Nothing here polls the database.
//
// Sessions are keyed by the parsed UUID rather than the string that arrived in
// the cookie, so a lookup is a fixed-size comparison and does not retain a
// request-scoped string. uuid.UUID is a [16]byte, so it compares with == and
// needs no wrapper type.
var liveSessions = struct {
	sync.Mutex
	sessions map[uuid.UUID]*liveSession
}{sessions: make(map[uuid.UUID]*liveSession)}

// trackClient records a connection under its session, arming the expiry timer on
// the first one. The timer is per session, so a dozen tabs cost one timer.
//
// A session that has already expired when its first connection arrives gets a
// timer that fires straight away, which closes the connection it just added. The
// connection cannot slip past revocation, because the callback cannot take the
// lock until trackClient has released it.
func trackClient(sessionID uuid.UUID, expiresAt time.Time, client *Client) {
	liveSessions.Lock()
	defer liveSessions.Unlock()

	session, ok := liveSessions.sessions[sessionID]
	if !ok {
		session = &liveSession{expiresAt: expiresAt, clients: make(map[*Client]struct{})}
		session.timer = time.AfterFunc(time.Until(expiresAt), func() {
			revokeSession(sessionID, CloseSessionRevoked, "session expired")
		})
		liveSessions.sessions[sessionID] = session
	}
	session.clients[client] = struct{}{}
}

// untrackClient drops a connection once it has finished. The session entry goes
// with its last connection, so nothing is left pointing at a timer that has no
// one to close.
func untrackClient(client *Client) {
	liveSessions.Lock()
	defer liveSessions.Unlock()

	session, ok := liveSessions.sessions[client.sessionID]
	if !ok {
		return
	}
	delete(session.clients, client)
	if len(session.clients) > 0 {
		return
	}
	session.timer.Stop()
	delete(liveSessions.sessions, client.sessionID)
}

// revokeSession ends a session: every connection authenticated by it is closed
// with a status the page can read, and the session is forgotten.
//
// The connections are collected under the lock but closed after releasing it.
// Closing one unblocks its readPump, whose deferred teardown calls
// untrackClient and wants the same lock, so closing them while holding it would
// deadlock.
func revokeSession(sessionID uuid.UUID, code int, reason string) {
	liveSessions.Lock()
	session, ok := liveSessions.sessions[sessionID]
	if !ok {
		liveSessions.Unlock()
		return
	}
	delete(liveSessions.sessions, sessionID)
	session.timer.Stop()

	clients := make([]*Client, 0, len(session.clients))
	for client := range session.clients {
		clients = append(clients, client)
	}
	session.clients = make(map[*Client]struct{})
	liveSessions.Unlock()

	for _, client := range clients {
		client.closeWith(code, reason)
	}
}

// RevokeSessionClients ends every WebSocket connection authenticated by a
// session, so logging out in one tab stops the sockets in all of them.
//
// The token arrives here as the canonical string that came out of the cookie,
// because that is the only form the caller has. gofrs/uuid parses a few
// surrounding forms too, so an unparseable token is the one case where nothing
// can be live: a value the server never issued has no registry entry to close.
func (h *Hub) RevokeSessionClients(sessionID string) {
	id, err := uuid.FromString(sessionID)
	if err != nil {
		return
	}
	revokeSession(id, CloseSessionRevoked, "session revoked")
}
