# Verifies the Gas Town panel loading spinner animates during a slow status
# fetch. Drive with a delayed fake supervisor:
#   go build -o /tmp/mg-fakegc ./testdata/fakegc
#   /tmp/mg-fakegc -addr :8088 -delay 8s &
#   vhs testdata/vhs/gastown-loading.tape
Output "/tmp/gastown-loading.gif"
Set Width 1600
Set Height 920
Set FontSize 16
Set Padding 12
Set Shell "bash"
Type "MG_GC_API=http://127.0.0.1:8088 MG_GC_CITY=bourbon ./mg --path testdata/screenshot.jsonl --no-animations"
Enter
Sleep 5s
Ctrl+g
Sleep 600ms
Screenshot "/tmp/gtload-1.png"
Sleep 500ms
Screenshot "/tmp/gtload-2.png"
Sleep 500ms
Screenshot "/tmp/gtload-3.png"
Sleep 8s
Screenshot "/tmp/gtload-done.png"
Sleep 300ms
