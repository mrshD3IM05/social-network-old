# Social Network

A Facebook-like social network: profiles, followers, posts with privacy, comments and reactions, groups with events, real-time chat and notifications.

- **Backend:** Go, SQLite, WebSocket
- **Frontend:** Next.js, React
- **Proxy:** Caddy

The API reference is in [backend/readme.md](backend/readme.md).

## Features

- Register, log in, log out (cookie sessions)
- Public or private profiles, followers and follow requests
- Posts with text and/or images: public, followers only, or chosen followers
- Comments and like/dislike reactions
- Groups: invitations, join requests, group posts, events and group chat
- Real-time private and group chat with images and "typing…"
- Live notifications

## How it works

```
                 browser
                    │
                    ▼
        ┌───────── Caddy :8000 ─────────┐
        │                               │
   /api/v1/*                      everything else
 (prefix removed)                       │
        │                               │
        ▼                               ▼
   Go API :8080                 Next.js :3000
   ├─ SQLite (sn.db)
   ├─ uploads/ (images)
   └─ /ws (WebSocket)
```

- **Caddy** is the single entry point. Requests to `/api/v1/...` go to the Go API with the prefix removed (`/api/v1/posts` → `/posts`). Everything else goes to Next.js. Because both share one origin, the session cookie works without any CORS setup.
- **Backend** is a plain `net/http` server. Each request goes through the layers: `handler` (parse and validate HTTP input) → `service` (business and privacy rules) → `repository` (SQL). The database file `sn.db` is created on first start, and the migrations in `internal/db/migrations/sqlite` run automatically.
- **Frontend** is a Next.js app that calls the API with `fetch` (see `frontend/lib/api.js`) and keeps one WebSocket open for the whole app (`frontend/lib/socket.js`), which reconnects with backoff if the connection drops.

### Sessions

Logging in creates a session stored in the database and sets an `HttpOnly` cookie named `session` that lasts 30 days. Every protected route, including the WebSocket, checks that cookie.

### Privacy

- **Public profile:** anyone can follow and see the posts.
- **Private profile:** following sends a request that the owner accepts or declines, and only followers see the profile content.
- **Post visibility:** public, followers only, or a hand-picked list of followers.
- **Groups:** posts, events and chat are visible to members only.
- **Direct messages:** you can message someone if you follow them or they follow you.

### Real-time

The WebSocket at `/api/v1/ws` pushes these events to the client:

| Event             | When                                              |
| ----------------- | ------------------------------------------------- |
| `message`         | a private or group message arrives                |
| `message_created` | your own message was saved (confirms the send)    |
| `typing`          | the other person is typing                        |
| `notification`    | follow request, group invite, join request, event |
| `error`           | a message could not be sent                       |

### Images

Posts, comments, messages and avatars accept JPEG, PNG or GIF images, up to 10 MB each and 3 per post, comment or message. Files are saved in `backend/uploads/` and served through `/api/v1/fs/{id}` with the same visibility rules as the thing they belong to.

### Limits

- 1000 requests per minute per IP overall
- 10 login/register attempts per minute per IP

## Run with Docker

The easiest way: you only need Docker.

```bash
docker compose up --build
```

Open http://localhost:8000

This starts three containers: `backend`, `frontend` (a production build) and `caddy`. Only Caddy is published on the host, on port 8000. The database and uploaded images are kept in the `backend-data` volume, so they survive restarts.

## Run locally

Requirements: Go 1.25+ (with a C compiler such as `gcc`, needed by the SQLite driver) and Node.js 22+.

### With the start script

```bash
./start.sh          # Linux
.\start-win.ps1     # Windows (PowerShell)
```

The script downloads Caddy into `caddy/` the first time, then starts Caddy, the backend and the frontend together. Open http://localhost:8000. Press `Ctrl+C` to stop everything.

### By hand

```bash
# terminal 1: API on :8080
cd backend
go run ./cmd/server

# terminal 2: web app on :3000
cd frontend
npm install
npm run dev
```

Open http://localhost:3000

Without Caddy, Next.js forwards `/api/v1/...` to `http://localhost:8080` itself (see `frontend/next.config.js`), and the WebSocket connects directly to `ws://localhost:8080/ws`.

### Ports

| Port | Service                    |
| ---- | -------------------------- |
| 8000 | Caddy (Docker or script)   |
| 8080 | Go API                     |
| 3000 | Next.js                    |

## Project structure

```
backend/
  cmd/server/            entry point: opens the DB, registers routes, starts the server
  internal/
    server/              list of all routes
    handler/             HTTP handlers, one package per feature
    service/             business rules and permission checks
    repository/          SQL queries
    model/               data structs
    middleware/          auth, rate limit, security headers
    websocket/           hub, live events, typing
    db/migrations/       SQL migrations (run on startup)
frontend/
  app/(auth)/            login and register pages
  app/(main)/            home, profile, people, groups, chat, notifications, settings
  components/            shared React components
  lib/                   API client, WebSocket, hooks
caddy/
  Caddyfile              proxy config for local runs
  Caddyfile.docker       proxy config for Docker
compose.yml              Docker Compose
start.sh / start-win.ps1 start everything locally
```

## Reset the database

Stop the backend first, then:

```bash
rm backend/sn.db backend/sn.db-wal backend/sn.db-shm
rm -rf backend/uploads
```

With Docker: `docker compose down -v`

## Troubleshooting

- **`go run` fails with a cgo or gcc error:** install a C compiler (`sudo apt install build-essential` on Debian/Ubuntu) and make sure `CGO_ENABLED=1`.
- **Port already in use:** another process is on 8000, 8080 or 3000. Stop it, or stop an earlier run of the start script.
- **Logged out after switching between :3000 and :8000:** the cookie belongs to one origin. Use the same URL the whole time.
- **Chat does not update live:** check the browser console for WebSocket errors. The banner at the top of the app shows when the connection is lost.
