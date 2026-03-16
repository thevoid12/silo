#!/usr/bin/env bash
# Verifies that a session_id sent in a follow-up chat request is accepted (session persistence).
set -euo pipefail

BASE_URL="${SILO_BASE_URL:-http://127.0.0.1:5110}"
TOKEN="${SILO_TOKEN:?SILO_TOKEN env var required}"
SESSION_ID="${SILO_SESSION_ID:?SILO_SESSION_ID env var required — run 03_chat_sse.sh first}"
TIMEOUT="${CHAT_TIMEOUT:-60}"

echo "==> [04] Session persistence — follow-up request reuses session_id"

RESPONSE=$(curl -s -N \
  --max-time "$TIMEOUT" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"message\":\"What did I just say?\",\"session_id\":\"${SESSION_ID}\"}" \
  "${BASE_URL}/silo/brain/chat" 2>&1 || true)

if [ -z "$RESPONSE" ]; then
  echo "FAIL: empty response from follow-up chat"
  exit 1
fi

echo "    raw SSE output:"
echo "$RESPONSE" | head -20

if ! echo "$RESPONSE" | grep -q "event: done"; then
  echo "FAIL: no 'event: done' in follow-up SSE stream"
  exit 1
fi

# Verify the same session_id is returned
RETURNED_ID=$(echo "$RESPONSE" | grep -A1 "event: done" | grep "data:" | head -1 | sed 's/.*"session_id":"\([^"]*\)".*/\1/')
if [ "$RETURNED_ID" != "$SESSION_ID" ]; then
  echo "FAIL: session_id mismatch — sent: $SESSION_ID, got: $RETURNED_ID"
  exit 1
fi

echo "    PASS: follow-up used same session_id: $SESSION_ID"
echo "PASS: session persistence works"
