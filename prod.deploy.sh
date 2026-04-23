#!/usr/bin/env bash
set -euo pipefail

REMOTE="${REMOTE:-user@example.com}"
REMOTE_DIR="${REMOTE_DIR:-/srv/apps/procyon}"
APP_NAME="${APP_NAME:-procyon-server}"
CONFIG_SOURCE="${CONFIG_SOURCE:-config/config.docker.json}"
REMOTE_CONFIG_NAME="${REMOTE_CONFIG_NAME:-config.docker.json}"

WITH_CONFIG=false
if [[ "${1:-}" == "--with-config" ]]; then
  WITH_CONFIG=true
fi

echo "==> Buduję binarkę lokalnie"
mkdir -p build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "build/$APP_NAME" ./

echo "==> Przygotowuję katalog na serwerze"
ssh "$REMOTE" "mkdir -p '$REMOTE_DIR'/build '$REMOTE_DIR'/config"

echo "==> Wysyłam build i pliki Dockera"
rsync -avz build/"$APP_NAME" "$REMOTE:$REMOTE_DIR/build/$APP_NAME"
rsync -avz Dockerfile compose.yaml .dockerignore "$REMOTE:$REMOTE_DIR/"
rsync -avz static/ "$REMOTE:$REMOTE_DIR/static/"

if $WITH_CONFIG; then
  echo "==> Wysyłam runtime config z $CONFIG_SOURCE"
  rsync -avz "$CONFIG_SOURCE" "$REMOTE:$REMOTE_DIR/config/$REMOTE_CONFIG_NAME"
fi

echo "==> Restartuję Dockera na serwerze"
ssh "$REMOTE" "cd '$REMOTE_DIR' && docker compose down --remove-orphans && docker compose up -d --build --remove-orphans"

echo "==> Status"
ssh "$REMOTE" "cd '$REMOTE_DIR' && docker compose ps"

echo "Deploy produkcyjny zakończony."
