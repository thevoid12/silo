#!/usr/bin/env bash
# Phase C end-to-end verification.
# Usage:
#   export SILO_TOKEN=<your-gateway-token>
#   export SILO_BASE_URL=http://127.0.0.1:5110   # optional, default shown
#   ./test/run_phase_c.sh
#
# The server must already be running: silo start
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

TOKEN="${SILO_TOKEN:?Set SILO_TOKEN to your gateway bearer token}"
export SILO_TOKEN="$TOKEN"
export SILO_BASE_URL="${SILO_BASE_URL:-http://127.0.0.1:5110}"

PASS=0
FAIL=0

run_test() {
  local script="$1"
  echo ""
  echo "------------------------------------------------------------"
  if bash "$script"; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
    echo "ERROR: $script failed"
  fi
}

run_test "${SCRIPT_DIR}/01_health.sh"
run_test "${SCRIPT_DIR}/02_auth.sh"

# Run chat SSE and capture session_id for subsequent tests
echo ""
echo "------------------------------------------------------------"
if SSE_OUTPUT=$(bash "${SCRIPT_DIR}/03_chat_sse.sh" 2>&1); then
  PASS=$((PASS + 1))
  echo "$SSE_OUTPUT"
  export SILO_SESSION_ID=$(echo "$SSE_OUTPUT" | grep "session_id:" | awk '{print $2}' | tr -d '[:space:]')
else
  FAIL=$((FAIL + 1))
  echo "ERROR: 03_chat_sse.sh failed"
fi

if [ -n "${SILO_SESSION_ID:-}" ]; then
  run_test "${SCRIPT_DIR}/04_session_persist.sh"
else
  echo ""
  echo "SKIP: 04_session_persist.sh — no session_id from chat test"
fi

run_test "${SCRIPT_DIR}/05_tool_approval.sh"

echo ""
echo "============================================================"
echo "Phase C results: ${PASS} passed, ${FAIL} failed"
echo "============================================================"
[ "$FAIL" -eq 0 ]
