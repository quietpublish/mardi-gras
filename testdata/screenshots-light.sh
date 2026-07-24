#!/bin/bash
# Regenerate the light-theme screenshots (docs/screenshots/light-theme*.png).
cd "$(dirname "$0")/.." && exec ./testdata/run-vhs.sh testdata/vhs/light.tape "docs/screenshots/light-theme.png + light-theme-roster.png"
