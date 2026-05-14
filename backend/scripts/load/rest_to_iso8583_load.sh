#!/usr/bin/env sh
set -eu

URL="${URL:-http://localhost:8080/cards/authorization}"
HOST_HEADER="${HOST_HEADER:-api.example.test}"
API_KEY="${API_KEY:-}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-100}"
BODY="${BODY:-{\"pan\":\"4111111111111111\",\"amount\":10000,\"currency\":\"IDR\",\"terminalId\":\"ATM00101\"}}"

if [ -z "$API_KEY" ]; then
  echo "API_KEY is required"
  exit 1
fi

seq "$REQUESTS" | xargs -I{} -P "$CONCURRENCY" sh -c '
  curl -sS -o /dev/null -w "%{http_code}\n" \
    -X POST "$URL" \
    -H "Host: $HOST_HEADER" \
    -H "Authorization: ApiKey $API_KEY" \
    -H "Content-Type: application/json" \
    --data "$BODY"
' sh
