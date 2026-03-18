#!/usr/bin/env bash
set -euo pipefail

VAULT_PASSWORD="thisisvoid"
PORT=5110
SERVER_LOG=$(mktemp)
SERVER_PID=""

cleanup() {
  [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
  rm -f "$SERVER_LOG"
}
trap cleanup EXIT

echo "==> starting silo server on port $PORT"
SILO_VAULT_PASSWORD="$VAULT_PASSWORD" ./silo start --serve --desktop-mode --port "$PORT" >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!

# poll until token: line appears or server exits
TOKEN=""
for i in $(seq 1 60); do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "ERROR: silo server exited early:"
    cat "$SERVER_LOG"
    exit 1
  fi
  TOKEN=$(grep -m1 "^token:" "$SERVER_LOG" 2>/dev/null | sed 's/^token://' || true)
  [ -n "$TOKEN" ] && break
  sleep 0.5
done

if [ -z "$TOKEN" ]; then
  echo "ERROR: silo server did not print token within 30s"
  cat "$SERVER_LOG"
  exit 1
fi

echo "==> server ready, running playwright tests"
cd desktop
SILO_DEV_PORT="$PORT" SILO_DEV_TOKEN="$TOKEN" bunx playwright test "$@"
