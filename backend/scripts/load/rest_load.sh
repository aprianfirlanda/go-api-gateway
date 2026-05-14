#!/usr/bin/env sh
set -eu

URL="${URL:-http://localhost:8080/accounts}"
HOST_HEADER="${HOST_HEADER:-api.example.test}"
API_KEY="${API_KEY:-}"
CONCURRENCY="${CONCURRENCY:-20}"
REQUESTS="${REQUESTS:-200}"
METHOD="${METHOD:-GET}"
BODY="${BODY:-}"

if [ -z "$API_KEY" ]; then
  echo "API_KEY is required"
  exit 1
fi

seq "$REQUESTS" | xargs -I{} -P "$CONCURRENCY" sh -c '
  if [ -n "$BODY" ]; then
    curl -sS -o /dev/null -w "%{http_code}\n" \
      -X "$METHOD" "$URL" \
      -H "Host: $HOST_HEADER" \
      -H "Authorization: ApiKey $API_KEY" \
      -H "Content-Type: application/json" \
      --data "$BODY"
  else
    curl -sS -o /dev/null -w "%{http_code}\n" \
      -X "$METHOD" "$URL" \
      -H "Host: $HOST_HEADER" \
      -H "Authorization: ApiKey $API_KEY"
  fi
' sh
