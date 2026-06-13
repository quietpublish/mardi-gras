# Captures the Detail panel (focused) showing the branded glamour markdown theme.
# Run from repo root after `make build`:
#   vhs testdata/vhs/detail.tape
Output "/tmp/detail-demo.gif"
Set Width 1800
Set Height 1040
Set FontSize 19
Set Padding 14
Set Shell "bash"
Type "./mg --path testdata/screenshot.jsonl --no-animations"
Enter
Sleep 6s
Tab
Sleep 1500ms
Screenshot "/tmp/detail-focused.png"
Sleep 300ms
