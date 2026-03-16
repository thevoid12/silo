#!/usr/bin/env bash
# Verifies the tool-approval endpoint: 404 for unknown id, 400 for missing request_id.
set -euo pipefail

BASE_URL="${SILO_BASE_URL:-http://127.0.0.1:5110}"
TOKEN="${SILO_TOKEN:?SILO_TOKEN env var required}"

echo "==> [05] Tool approval endpoint"

# Missing request_id -> 400
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"approved":true}' \
  "${BASE_URL}/silo/brain/tool-approval")
if [ "$STATUS" != "400" ]; then
  echo "FAIL: missing request_id: expected 400, got $STATUS"
  exit 1
fi
echo "    PASS: missing request_id -> 400"

# Unknown request_id -> 404
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"request_id":"no-such-id","approved":true}' \
  "${BASE_URL}/silo/brain/tool-approval")
if [ "$STATUS" != "404" ]; then
  echo "FAIL: unknown request_id: expected 404, got $STATUS"
  exit 1
fi
echo "    PASS: unknown request_id -> 404"

# No auth -> 401
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -d '{"request_id":"x","approved":true}' \
  "${BASE_URL}/silo/brain/tool-approval")
if [ "$STATUS" != "401" ]; then
  echo "FAIL: no-auth: expected 401, got $STATUS"
  exit 1
fi
echo "    PASS: no-auth -> 401"

echo "PASS: tool-approval endpoint checks complete"
