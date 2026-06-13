#!/bin/bash
# Regenerate the README hero GIF: builds mg + the fake supervisor, starts
# fakegc, drives mg through the parade → detail → Gas Town flow via vhs, and
# writes docs/screenshots/demo.gif.
# Requires vhs (brew install vhs ffmpeg ttyd). Run from the repo root.
set -euo pipefail
command -v vhs >/dev/null 2>&1 || { echo "vhs not installed — brew install vhs ffmpeg ttyd"; exit 1; }
go build -o ./mg ./cmd/mg
go build -o /tmp/mg-fakegc ./testdata/fakegc
/tmp/mg-fakegc -addr :8088 >/tmp/mg-fakegc.log 2>&1 &
trap 'kill $! 2>/dev/null || true' EXIT
sleep 1
vhs testdata/vhs/demo.tape
echo "wrote docs/screenshots/demo.gif"
