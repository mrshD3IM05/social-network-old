#!/usr/bin/env bash

set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CADDY_DIR="$ROOT/caddy"
CADDY_BIN="$CADDY_DIR/caddy"
CADDY_CONFIG="$CADDY_DIR/Caddyfile"

echo "================================="
echo " Starting Social Network"
echo "================================="

# -------------------------------
# Caddy
# -------------------------------

if [ ! -f "$CADDY_BIN" ]; then
    echo "Caddy not found. Downloading..."

    mkdir -p "$CADDY_DIR"

    curl -L \
        "https://caddyserver.com/api/download?os=linux&arch=amd64" \
        -o "$CADDY_BIN"

    chmod +x "$CADDY_BIN"

    echo "Caddy downloaded."
fi

# -------------------------------
# Start Caddy
# -------------------------------

echo "Starting Caddy..."

cd "$CADDY_DIR"

"$CADDY_BIN" run \
    --config "$CADDY_CONFIG" &

CADDY_PID=$!

# -------------------------------
# Start Backend
# -------------------------------

echo "Starting backend..."

cd "$ROOT/backend"

go run ./cmd/server &

BACKEND_PID=$!

# -------------------------------
# Start Frontend
# -------------------------------

echo "Starting frontend..."

cd "$ROOT/frontend"

npm ci
npm run dev &

FRONTEND_PID=$!

echo ""
echo "================================="
echo " Everything started!"
echo "================================="
echo ""
echo "Caddy PID:    $CADDY_PID"
echo "Backend PID:  $BACKEND_PID"
echo "Frontend PID: $FRONTEND_PID"

wait