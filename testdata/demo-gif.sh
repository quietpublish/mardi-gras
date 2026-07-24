#!/bin/bash
# Regenerate the README hero GIF (docs/screenshots/demo.gif).
cd "$(dirname "$0")/.." && exec ./testdata/run-vhs.sh testdata/vhs/demo.tape "docs/screenshots/demo.gif"
