#!/usr/bin/env bash
# Verifies bearer token auth: 401 without token, 401 with wrong token, 200 with correct token.
set -euo pipefail

BASE_URL="${SILO_BASE_URL:-http://127.0.0.1:5110}"
TOKEN="${SILO_TOKEN:?SILO_TOKEN env var required}"

echo "==> [02] Auth checks"

# No token -> 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/silo/status")
if [ "$STATUS" != "401" ]; then
  echo "FAIL: no-token: expected 401, got $STATUS"
  exit 1
fi
echo "    PASS no-token -> 401"

# Wrong token -> 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer wrong-token" "${BASE_URL}/silo/status")
if [ "$STATUS" != "401" ]; then
  echo "FAIL: wrong-token: expected 401, got $STATUS"
  exit 1
fi
echo "    PASS wrong-token -> 401"

# Correct token -> 200
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/silo/status")
if [ "$STATUS" != "200" ]; then
  echo "FAIL: valid-token: expected 200, got $STATUS"
  exit 1
fi
BODY=$(curl -s -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/silo/status")
echo "    response: $BODY"
echo "    PASS valid-token -> 200"

echo "PASS: auth checks complete"
