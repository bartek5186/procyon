#!/usr/bin/env bash
set -euo pipefail

APP_NAME="procyon-server"

echo "==> Buduję binarkę lokalnie"
mkdir -p build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "build/$APP_NAME" ./

echo "==> Restartuję Dockera lokalnie"
docker compose down --remove-orphans
docker compose up -d --build --remove-orphans

echo "==> Status"
docker compose ps

echo "Deploy lokalny zakończony."
