#!/usr/bin/env bash
# Verifies GET /health returns 200 OK (no auth required)
set -euo pipefail

BASE_URL="${SILO_BASE_URL:-http://127.0.0.1:5110}"

echo "==> [01] Health check"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health")
if [ "$STATUS" != "200" ]; then
  echo "FAIL: expected 200, got $STATUS"
  exit 1
fi

BODY=$(curl -s "${BASE_URL}/health")
echo "    response: $BODY"
echo "PASS: /health returned 200"
