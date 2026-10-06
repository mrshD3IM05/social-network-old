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

## Run with Docker

```bash
docker compose up --build
```

Open http://localhost:8000

## Run locally

Requirements: Go 1.25+ (with a C compiler for SQLite) and Node.js 22+.

```bash
# terminal 1
cd backend
go run ./cmd/server

# terminal 2
cd frontend
npm install
npm run dev
```

Open http://localhost:3000

You can also use `./start.sh` (Linux) or `start-win.ps1` (Windows).

## Project structure

```
backend/    Go API: handlers, services, repository, migrations, websocket
frontend/   Next.js app: pages, components, lib
caddy/      Caddy config
compose.yml Docker Compose
```

## Reset the database

```bash
rm backend/sn.db backend/sn.db-wal backend/sn.db-shm
rm -rf backend/uploads
```

With Docker: `docker compose down -v`
