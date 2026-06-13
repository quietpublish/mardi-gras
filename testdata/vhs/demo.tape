# Hero demo GIF for the README. Drives the core flow against the fake Gas City
# supervisor so the Gas Town panel has real data. Regenerate via `make demo-gif`
# (which builds mg + fakegc and starts the supervisor), or manually:
#   go build -o /tmp/mg-fakegc ./testdata/fakegc
#   /tmp/mg-fakegc -addr :8088 &
#   vhs testdata/vhs/demo.tape
Output "docs/screenshots/demo.gif"
Set Width 1500
Set Height 860
Set FontSize 15
Set Padding 12
Set Framerate 24
Set Shell "bash"

Type "MG_GC_API=http://127.0.0.1:8088 MG_GC_CITY=bourbon ./mg --path testdata/screenshot.jsonl"
Enter
Sleep 6s
Screenshot "/tmp/demo-1-parade.png"

# Browse the parade.
Down@550ms
Down@550ms
Down@550ms
Up@550ms
Up@550ms
Up@550ms
Sleep 600ms

# Focus the detail pane — gold focus border + Mardi Gras markdown theme —
# and scroll through the themed description.
Tab
Sleep 1200ms
Down@350ms
Down@350ms
Down@350ms
Down@350ms
Sleep 800ms
Screenshot "/tmp/demo-2-detail.png"
Sleep 800ms

# Open the Gas Town panel — the agent roster, convoys, and mail.
Tab
Sleep 300ms
Ctrl+g
Sleep 2500ms
Screenshot "/tmp/demo-3-roster.png"
Sleep 2500ms
