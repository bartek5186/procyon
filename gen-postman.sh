#!/usr/bin/env sh

# Wczytanie zmiennych z pliku .env
if [ -f .env ]; then
    # xargs w czystym sh może wymagać flagi -d '\n' przy skomplikowanych wartościach
    export $(grep -v '^#' .env | xargs)
fi

# Usunięto pipefail, zostawiono -eu dla bezpieczeństwa
set -eu

OUT="${POSTMAN_COLLECTION_FILE:-docs/json/PostmanCollection.generated.json}"

GOCACHE=/tmp/procyon-go-build-cache go run ./tools/postman-gen \
  -root . \
  -out "$OUT" \
  -base-url "${POSTMAN_BASE_URL:-}" \
  -admin-url "${POSTMAN_ADMIN_URL:-}" \
  -upload-url "${POSTMAN_UPLOAD_URL:-}" \
  -admin-key "${POSTMAN_ADMIN_KEY:-}" \
  -auth-key "${POSTMAN_AUTH_KEY:-}"

primary_complete=0
secondary_complete=0
secondary_collection_id="${POSTMAN_API_COLLECTION_ID_1:-${POSTMAN_COLLECTION_ID_1:-}}"

if [ -n "${POSTMAN_API_KEY:-}" ] && [ -n "${POSTMAN_COLLECTION_ID:-}" ]; then
  primary_complete=1
elif [ -n "${POSTMAN_API_KEY:-}" ] || [ -n "${POSTMAN_COLLECTION_ID:-}" ]; then
  echo "Primary Postman upload skipped. Set both POSTMAN_API_KEY and POSTMAN_COLLECTION_ID." >&2
fi

if [ -n "${POSTMAN_API_KEY_1:-}" ] && [ -n "$secondary_collection_id" ]; then
  secondary_complete=1
elif [ -n "${POSTMAN_API_KEY_1:-}" ] || [ -n "$secondary_collection_id" ]; then
  echo "Secondary Postman upload skipped. Set POSTMAN_API_KEY_1 and POSTMAN_API_COLLECTION_ID_1 or POSTMAN_COLLECTION_ID_1." >&2
fi

if [ "$primary_complete" -eq 0 ] && [ "$secondary_complete" -eq 0 ]; then
  echo "Postman upload skipped. Set POSTMAN_API_KEY/POSTMAN_COLLECTION_ID or POSTMAN_API_KEY_1 with POSTMAN_API_COLLECTION_ID_1/POSTMAN_COLLECTION_ID_1."
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "Postman upload requires jq, but jq was not found." >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "Postman upload requires curl, but curl was not found." >&2
  exit 1
fi

upload_postman_collection() {
  api_key="$1"
  collection_id="$2"
  label="$3"

  echo "Uploading $OUT to Postman collection $collection_id ($label)"
  jq -n --slurpfile c "$OUT" \
    '{ collection: $c[0] }' \
  | curl --request PUT \
    --fail \
    --location "https://api.getpostman.com/collections/$collection_id" \
    --header "X-API-Key: $api_key" \
    --header "Content-Type: application/json" \
    --data @-

  echo "Postman collection updated ($label)."
}

if [ "$primary_complete" -eq 1 ]; then
  upload_postman_collection "$POSTMAN_API_KEY" "$POSTMAN_COLLECTION_ID" "primary"
fi

if [ "$secondary_complete" -eq 1 ]; then
  upload_postman_collection "$POSTMAN_API_KEY_1" "$secondary_collection_id" "secondary"
fi
