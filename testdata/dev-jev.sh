#!/bin/bash
# dev-jev.sh — run mg against the fake Jev server (testdata/fakejev) to see the
# Jev plumbing without a TypeSafe key: the footer chip, the startup probe, the
# circuit breaker, and (once a feature registers questions) what mg sends.
# Invoked by `make dev-jev`; assumes ./mg is already built.
#
# Usage: ./testdata/dev-jev.sh [extra mg flags]
#   ADDR=:9091 ./testdata/dev-jev.sh                 # another port
#   FAKEJEV_FLAGS="-status 401" ./testdata/dev-jev.sh # watch Jev disable itself
#   FAKEJEV_FLAGS="-fail-every 2" ./testdata/dev-jev.sh
# The fake logs every request's shape to /tmp/mg-fakejev.log (tail -f it).
set -euo pipefail

ADDR="${ADDR:-:8091}"

go build -o /tmp/mg-fakejev ./testdata/fakejev
# shellcheck disable=SC2086 # FAKEJEV_FLAGS is a flag list by design
/tmp/mg-fakejev -addr "$ADDR" ${FAKEJEV_FLAGS:-} >/tmp/mg-fakejev.log 2>&1 &
FAKEJEV=$!
trap 'kill "$FAKEJEV" 2>/dev/null || true' EXIT
sleep 1

echo "fakejev on http://127.0.0.1${ADDR}; log: /tmp/mg-fakejev.log"
echo "Look for the 'jev' chip in the footer. Tip: 120x38 terminal for screenshots."
MG_JEV_URL="http://127.0.0.1${ADDR}" MG_JEV_API_KEY="fake" ./mg --path testdata/screenshot.jsonl "$@"
