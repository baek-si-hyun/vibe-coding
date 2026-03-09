#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

PORT="${NEWS_GO_PORT:-${BACKEND_PORT:-5002}}"
if [[ -f .env ]]; then
  env_port="$(grep -E '^(NEWS_GO_PORT|BACKEND_PORT)=' .env | tail -n1 | cut -d'=' -f2- || true)"
  env_port="${env_port//\"/}"
  env_port="${env_port//\'/}"
  env_port="${env_port//[[:space:]]/}"
  if [[ -n "${env_port}" ]]; then
    PORT="${env_port}"
  fi
fi

if lsof -PiTCP:"${PORT}" -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo "[backend-go] port ${PORT} is already in use; reusing existing server."
  echo "[backend-go] if you want to run a new one, stop the existing process first."
  trap 'exit 0' INT TERM
  while true; do
    sleep 3600 &
    wait $!
  done
fi

exec go run ./cmd/backend
