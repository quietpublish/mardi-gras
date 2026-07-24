# Light theme screenshot (issue #86). Drives mg with MG_THEME=light on a light
# terminal theme against the fake Gas City supervisor. Regenerate via
# `make screenshot-light`, or manually:
#   go build -o /tmp/mg-fakegc ./testdata/fakegc
#   /tmp/mg-fakegc -addr :8088 &
#   vhs testdata/vhs/light.tape
Output "/tmp/mg-light.gif"
Set Width 1500
Set Height 860
Set FontSize 15
Set Padding 12
Set Shell "bash"
Set Theme "Builtin Light"

Type "MG_THEME=light MG_GC_API=http://127.0.0.1:8088 MG_GC_CITY=bourbon ./mg --path testdata/screenshot.jsonl"
Enter
Sleep 6s

# Focus the detail pane so the hero shot shows the themed markdown.
Tab
Sleep 1200ms
Down@350ms
Down@350ms
Down@350ms
Sleep 800ms
Screenshot "docs/screenshots/light-theme.png"

# Gas Town roster on the light palette.
Tab
Sleep 300ms
Ctrl+g
Sleep 2500ms
Screenshot "docs/screenshots/light-theme-roster.png"
Sleep 500ms
