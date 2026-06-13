# Regenerates the Gas City screenshots against the fake supervisor (testdata/fakegc).
# Run via `make screenshots-gc` (which starts fakegc first), or:
#   go build -o /tmp/mg-fakegc ./testdata/fakegc && /tmp/mg-fakegc -addr :8088 &
#   vhs testdata/vhs/gascity.tape
Output "/tmp/gascity-demo.gif"
Set Width 1540
Set Height 880
Set FontSize 14
Set Padding 12
Set Shell "bash"
Type "MG_GC_API=http://127.0.0.1:8088 MG_GC_CITY=bourbon ./mg --path testdata/screenshot.jsonl --no-animations"
Enter
Sleep 4s
Ctrl+g
Sleep 3500ms
Screenshot "docs/screenshots/gascity-roster.png"
Ctrl+g
Sleep 700ms
Type "a"
Sleep 1200ms
Screenshot "docs/screenshots/gascity-sling-target.png"
Sleep 300ms
