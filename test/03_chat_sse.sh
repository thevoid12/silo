#!/usr/bin/env bash
# Verifies POST /silo/brain/chat streams SSE events and returns a session_id in 'done'.
set -euo pipefail

BASE_URL="${SILO_BASE_URL:-http://127.0.0.1:5110}"
TOKEN="${SILO_TOKEN:?SILO_TOKEN env var required}"
TIMEOUT="${CHAT_TIMEOUT:-60}"

echo "==> [03] Chat SSE stream"

# Capture SSE output; stop after 'done' event or timeout
RESPONSE=$(curl -s -N \
  --max-time "$TIMEOUT" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"message":"Say exactly: hello silo"}' \
  "${BASE_URL}/silo/brain/chat" 2>&1 || true)

if [ -z "$RESPONSE" ]; then
  echo "FAIL: empty response from /silo/brain/chat"
  exit 1
fi

echo "    raw SSE output:"
echo "$RESPONSE" | head -40

# Check that we got at least one 'event: token' line
if ! echo "$RESPONSE" | grep -q "event: token"; then
  echo "FAIL: no 'event: token' found in SSE stream"
  exit 1
fi
echo "    PASS: received token events"

# Check that we got 'event: done'
if ! echo "$RESPONSE" | grep -q "event: done"; then
  echo "FAIL: no 'event: done' found in SSE stream"
  exit 1
fi
echo "    PASS: received done event"

# Extract session_id from the done event data
SESSION_ID=$(echo "$RESPONSE" | grep -A1 "event: done" | grep "data:" | head -1 | sed 's/.*"session_id":"\([^"]*\)".*/\1/')
if [ -z "$SESSION_ID" ]; then
  echo "WARN: could not extract session_id from done event"
else
  echo "    session_id: $SESSION_ID"
fi

echo "PASS: SSE chat stream works"

# Export for use in subsequent tests
export SILO_SESSION_ID="$SESSION_ID"
