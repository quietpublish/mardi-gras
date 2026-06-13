#!/bin/bash
# dev-gc.sh — run mg against the fake Gas City supervisor (testdata/fakegc) for
# local demos and screenshots, without a real `gc` install. The HTTP analogue
# of `make dev-gt`. Invoked by `make dev-gc`; assumes ./mg is already built.
#
# Usage: ./testdata/dev-gc.sh [extra mg flags]
#   ADDR=:9090 ./testdata/dev-gc.sh        # override the fake supervisor port
set -euo pipefail

ADDR="${ADDR:-:8088}"
CITY="bourbon"

go build -o /tmp/mg-fakegc ./testdata/fakegc
/tmp/mg-fakegc -addr "$ADDR" >/tmp/mg-fakegc.log 2>&1 &
FAKEGC=$!
trap 'kill "$FAKEGC" 2>/dev/null || true' EXIT
sleep 1

echo "fakegc on http://127.0.0.1${ADDR} (city ${CITY}); log: /tmp/mg-fakegc.log"
echo "Press ctrl+g for the Gas City panel. Tip: 120x38 terminal for screenshots."
MG_GC_API="http://127.0.0.1${ADDR}" MG_GC_CITY="$CITY" ./mg --path testdata/screenshot.jsonl "$@"
