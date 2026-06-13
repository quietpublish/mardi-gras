// Command fakegc is a fake Gas City supervisor for local TUI testing and
// screenshots. It serves canned, screenshot-worthy data for the /v0 endpoints
// mg's GCDriver consumes, so you can drive the Gas City panel without a real
// `gc` install. It is the HTTP analogue of testdata/fake-gt.sh.
//
//	go run ./testdata/fakegc -addr :8088
//	MG_GC_API=http://127.0.0.1:8088 MG_GC_CITY=bourbon ./mg --path testdata/screenshot.jsonl
//
// Lives under testdata/ so the Go toolchain ignores it (not built/linted/shipped
// with the module). See `make dev-gc`.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const city = "bourbon"

// All responses are canned JSON literals matching the gcclient schema field
// names. Theme: a New Orleans krewe of agents working a release.
var responses = map[string]string{
	// Supervisor: one running city.
	"/v0/cities": `{"items":[
		{"name":"bourbon","path":"/Users/you/gc/bourbon","running":true,"status":"running","phases_completed":["loading_config","starting_bead_store","ready"]}
	],"total":1}`,

	// Roster — varied roles, states, providers/models, hook beads, activity.
	"/v0/city/bourbon/agents": `{"items":[
		{"name":"zulu","pool":"polecat","rig":"krewe","running":true,"suspended":false,"available":true,"state":"working","active_bead":"mg-q12","activity":"refactoring the auth service","provider":"anthropic","model":"opus","display_name":"Zulu","context_pct":63,"session":{"name":"zulu","attached":true}},
		{"name":"rex","pool":"polecat","rig":"krewe","running":true,"suspended":false,"available":true,"state":"working","active_bead":"mg-q18","activity":"writing table-driven tests","provider":"anthropic","model":"sonnet","display_name":"Rex","context_pct":38,"session":{"name":"rex","attached":true}},
		{"name":"orpheus","pool":"polecat","rig":"krewe","running":true,"suspended":false,"available":true,"state":"backoff","active_bead":"mg-q07","activity":"retrying a flaky merge","provider":"anthropic","model":"opus","display_name":"Orpheus","context_pct":81,"session":{"name":"orpheus","attached":false}},
		{"name":"bacchus","pool":"crew","rig":"krewe","running":true,"suspended":false,"available":true,"state":"idle","provider":"openai","model":"codex","display_name":"Bacchus"},
		{"name":"proteus","pool":"witness","rig":"krewe","running":true,"suspended":false,"available":true,"state":"patrolling","activity":"watching for stalls","provider":"anthropic","model":"haiku","display_name":"Proteus"},
		{"name":"muses","pool":"polecat","rig":"second_line","running":true,"suspended":false,"available":true,"state":"awaiting-gate","active_bead":"mg-s03","activity":"waiting on review quorum","provider":"anthropic","model":"opus","display_name":"Muses","context_pct":52,"session":{"name":"muses","attached":true}},
		{"name":"endymion","pool":"refinery","rig":"second_line","running":true,"suspended":false,"available":true,"state":"idle","provider":"anthropic","model":"sonnet","display_name":"Endymion"},
		{"name":"nyx","pool":"polecat","rig":"second_line","running":true,"suspended":false,"available":true,"state":"spawning","activity":"booting up","provider":"openai","model":"codex","display_name":"Nyx"}
	],"total":8,"partial":false}`,

	// Sessions — back the running agents so nudge/decommission resolve.
	"/v0/city/bourbon/sessions": `{"items":[
		{"id":"s-zulu","session_name":"zulu","title":"zulu","pool":"polecat","rig":"krewe","state":"active","provider":"anthropic","template":"polecat","created_at":"2026-06-13T08:00:00Z","attached":true,"running":true,"display_name":"Zulu"},
		{"id":"s-rex","session_name":"rex","title":"rex","pool":"polecat","rig":"krewe","state":"active","provider":"anthropic","template":"polecat","created_at":"2026-06-13T08:05:00Z","attached":true,"running":true,"display_name":"Rex"},
		{"id":"s-orpheus","session_name":"orpheus","title":"orpheus","pool":"polecat","rig":"krewe","state":"active","provider":"anthropic","template":"polecat","created_at":"2026-06-13T08:10:00Z","attached":false,"running":true,"display_name":"Orpheus"},
		{"id":"s-muses","session_name":"muses","title":"muses","pool":"polecat","rig":"second_line","state":"active","provider":"anthropic","template":"polecat","created_at":"2026-06-13T08:20:00Z","attached":true,"running":true,"display_name":"Muses"}
	],"total":4}`,

	// Mail — a mix of read/unread, priorities, senders.
	"/v0/city/bourbon/mail": `{"items":[
		{"id":"m1","from":"witness","to":"mayor","subject":"Quorum reached on mg-q12","body":"Two approvals in. Safe to land.","created_at":"2026-06-13T09:30:00Z","read":false,"priority":1,"rig":"krewe"},
		{"id":"m2","from":"orpheus","to":"mayor","subject":"Backoff: merge conflict on mg-q07","body":"Hit a conflict in auth.go; backing off and retrying.","created_at":"2026-06-13T09:12:00Z","read":false,"priority":2,"rig":"krewe"},
		{"id":"m3","from":"controller","to":"mayor","subject":"Convoy 'Carnival release' is 60% complete","body":"3 of 5 beads landed.","created_at":"2026-06-13T08:45:00Z","read":true,"priority":3},
		{"id":"m4","from":"rex","to":"zulu","subject":"Tests green on the auth refactor branch","body":"All 41 tests pass. Handing back.","created_at":"2026-06-13T08:30:00Z","read":true,"priority":3,"rig":"krewe"}
	],"total":4}`,

	// Formulas — for the sling-with-formula picker.
	"/v0/city/bourbon/formulas": `{"items":[
		{"name":"shiny","description":"design -> implement -> review -> test -> submit","version":"3","var_defs":[],"run_count":47,"recent_runs":[]},
		{"name":"quick","description":"implement -> submit (no review)","version":"2","var_defs":[],"run_count":112,"recent_runs":[]},
		{"name":"review-quorum","description":"implement -> 2-reviewer quorum -> submit","version":"1","var_defs":[],"run_count":8,"recent_runs":[]},
		{"name":"hotfix","description":"patch -> fast-track submit","version":"1","var_defs":[],"run_count":3,"recent_runs":[]}
	],"total":4,"partial":false}`,

	// Convoys — beads of issue_type "convoy".
	"/v0/city/bourbon/convoys": `{"items":[
		{"id":"cv-carnival","title":"Carnival release","status":"open","issue_type":"convoy","priority":1},
		{"id":"cv-cleanup","title":"Tech-debt cleanup","status":"open","issue_type":"convoy","priority":3}
	],"total":2}`,

	// One rich convoy detail (children + progress) for the expanded view.
	"/v0/city/bourbon/convoy/cv-carnival": `{
		"convoy":{"id":"cv-carnival","title":"Carnival release","status":"open","issue_type":"convoy","priority":1},
		"children":[
			{"id":"mg-q12","title":"Refactor auth service","status":"in_progress","issue_type":"task","assignee":"zulu"},
			{"id":"mg-q18","title":"Add auth test coverage","status":"in_progress","issue_type":"task","assignee":"rex"},
			{"id":"mg-q07","title":"Fix flaky merge on release branch","status":"open","issue_type":"task","assignee":"orpheus"},
			{"id":"mg-q21","title":"Update changelog","status":"closed","issue_type":"task"},
			{"id":"mg-q22","title":"Cut release tag","status":"closed","issue_type":"task"}
		],
		"progress":{"total":5,"closed":2}
	}`,
}

func main() {
	addr := flag.String("addr", ":8088", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	// Canned GETs.
	for path, body := range responses {
		body := body
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			logReq(r)
			writeJSON(w, http.StatusOK, body)
		})
	}

	// Everything else: accept mutations (POST/PUT/PATCH/DELETE) with a 2xx so
	// sling/nudge/decommission/convoy/mail actions succeed in the demo, and
	// 404 unknown GETs (mg treats those as "feature absent").
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logReq(r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusNotFound, `{"title":"Not Found","status":404,"detail":"not_found: fakegc has no canned response for this path"}`)
		default: // mutation
			code := http.StatusOK
			if strings.HasSuffix(r.URL.Path, "/submit") || strings.HasSuffix(r.URL.Path, "/kill") {
				code = http.StatusAccepted
			} else if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "convoys") {
				code = http.StatusCreated
				writeJSON(w, code, `{"id":"cv-new","title":"new convoy","status":"open","issue_type":"convoy"}`)
				return
			}
			writeJSON(w, code, `{"status":"accepted"}`)
		}
	})

	log.Printf("fakegc: Supervisor API listening on http://127.0.0.1%s (city %q)", normalize(*addr), city)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = fmt.Fprint(w, body)
}

func logReq(r *http.Request) { log.Printf("fakegc: %s %s", r.Method, r.URL.Path) }

func normalize(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + strings.TrimPrefix(addr, "127.0.0.1:")
}
