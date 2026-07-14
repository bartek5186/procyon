#!/usr/bin/env sh

set -eu

if ! command -v procyon-cli >/dev/null 2>&1; then
  echo "Postman generation requires procyon-cli. Install it with: go install github.com/bartek5186/procyon-cli@latest" >&2
  exit 1
fi

procyon-cli postman sync --root .
