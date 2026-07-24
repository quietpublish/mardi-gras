#!/bin/bash
# Regenerate the Gas City screenshots (docs/screenshots/gascity-*.png).
cd "$(dirname "$0")/.." && exec ./testdata/run-vhs.sh testdata/vhs/gascity.tape "docs/screenshots/gascity-roster.png + gascity-sling-target.png"
