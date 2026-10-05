// Command fakejev is a fake System One (Jev) server for local TUI testing.
// It speaks the wire format mg's internal/jev client sends and answers every
// question deterministically from a hash of the question, so screenshots and
// manual runs are stable. It logs each request's shape (never the API key) so
// you can eyeball what mg is willing to send off-machine.
//
//	go run ./testdata/fakejev -addr :8091
//	MG_JEV_URL=http://127.0.0.1:8091 MG_JEV_API_KEY=fake ./mg --path testdata/screenshot.jsonl
//
// Flags: -latency adds a delay per request; -fail-every N answers every Nth
// request with 503 (to watch the circuit breaker); -status N answers every
// request with that status (401 to see Jev disable itself); -dump logs the
// full request JSON.
//
// Lives under testdata/ so the Go toolchain ignores it (not built, linted or
// shipped with the module). See `make dev-jev`.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type question struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria"`
}

type request struct {
	State     json.RawMessage     `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]question `json:"questions"`
}

// answer mirrors the hosted API's shape as observed against jev-1.13.0: a
// noul carries only its probability (no confidence, no distribution), a
// score's probabilities are keyed by level index with a legend, and a
// choice's are keyed by option name. mg's client normalizes all three.
type answer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

type response struct {
	Model   string            `json:"model"`
	Answers map[string]answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

var (
	latency   = flag.Duration("latency", 150*time.Millisecond, "delay before each answer")
	failEvery = flag.Int("fail-every", 0, "answer every Nth request with 503 (0 = never)")
	status    = flag.Int("status", 0, "answer every request with this HTTP status (0 = normal)")
	dump      = flag.Bool("dump", false, "log the full request JSON")
	requests  atomic.Int64
)

func main() {
	addr := flag.String("addr", ":8091", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/systemone", handle)
	log.Printf("fakejev listening on %s (latency %s)", *addr, *latency)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func handle(w http.ResponseWriter, r *http.Request) {
	n := requests.Add(1)
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		log.Printf("#%d 401: no bearer token", n)
		http.Error(w, `{"error":"missing api key"}`, http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("#%d 400: %v", n, err)
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}
	logRequest(n, body, req)

	time.Sleep(*latency)
	if *status != 0 {
		log.Printf("#%d -> forced %d", n, *status)
		http.Error(w, fmt.Sprintf(`{"error":"forced %d"}`, *status), *status)
		return
	}
	if *failEvery > 0 && n%int64(*failEvery) == 0 {
		log.Printf("#%d -> 503 (fail-every %d)", n, *failEvery)
		w.Header().Set("Retry-After", "1")
		http.Error(w, `{"error":"overloaded"}`, http.StatusServiceUnavailable)
		return
	}

	resp := response{Model: req.Model, Answers: make(map[string]answer, len(req.Questions))}
	for key, q := range req.Questions {
		resp.Answers[key] = answerFor(key, q)
	}
	resp.Usage.InputTokens = len(body) / 4
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("#%d write: %v", n, err)
	}
}

// logRequest prints the shape of what mg sent: the top-level state keys,
// the issue IDs when the state is a chunk of issues, and the question names.
func logRequest(n int64, body []byte, req request) {
	var state map[string]json.RawMessage
	_ = json.Unmarshal(req.State, &state)
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	shape := "state{" + strings.Join(keys, ",") + "}"
	if raw, ok := state["issues"]; ok {
		var issues []struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &issues)
		ids := make([]string, len(issues))
		for i, iss := range issues {
			ids[i] = iss.ID
		}
		shape += fmt.Sprintf(" issues=%d [%s]", len(issues), strings.Join(ids, " "))
	}
	names := map[string]int{}
	for key := range req.Questions {
		name := key
		if i := strings.LastIndexByte(key, '/'); i >= 0 {
			name = key[i+1:]
		}
		names[name]++
	}
	var qs []string
	for name, c := range names {
		qs = append(qs, fmt.Sprintf("%s×%d", name, c))
	}
	sort.Strings(qs)
	log.Printf("#%d %d bytes model=%s %s questions{%s}", n, len(body), req.Model, shape, strings.Join(qs, " "))
	if *dump {
		log.Printf("#%d body: %s", n, body)
	}
}

// answerFor derives a stable answer from the question key and text.
func answerFor(key string, q question) answer {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte(q.Instructions))
	seed := h.Sum32()
	unit := func(salt uint32) float64 { return float64((seed^salt)%1000) / 1000 }
	a := answer{Type: q.Type}

	switch q.Type {
	case "noul":
		a.Noul = unit(1)
	case "choice":
		names := optionNames(q.Criteria)
		if len(names) == 0 {
			break
		}
		a.Probabilities = spread(names, seed)
		a.Choice = argmax(a.Probabilities)
		a.Confidence = a.Probabilities[a.Choice]
	case "score":
		var levels []string
		_ = json.Unmarshal(q.Criteria, &levels)
		if len(levels) == 0 {
			break
		}
		byLabel := spread(levels, seed)
		a.Probabilities = make(map[string]float64, len(levels))
		a.Legend = make(map[string]string, len(levels))
		for i, l := range levels {
			k := strconv.Itoa(i)
			a.Probabilities[k] = byLabel[l]
			a.Legend[k] = l
			a.Score += float64(i) * byLabel[l]
			a.Confidence = max(a.Confidence, byLabel[l])
		}
	}
	return a
}

// optionNames reads choice criteria in either documented form: an object of
// name -> description, or a list of names.
func optionNames(raw json.RawMessage) []string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		names := make([]string, 0, len(obj))
		for k := range obj {
			names = append(names, k)
		}
		sort.Strings(names)
		return names
	}
	var list []string
	_ = json.Unmarshal(raw, &list)
	return list
}

// spread assigns labels probabilities that sum to one, peaked on one label
// chosen from the seed.
func spread(labels []string, seed uint32) map[string]float64 {
	out := make(map[string]float64, len(labels))
	peak := int(seed % uint32(len(labels)))
	var total float64
	for i, l := range labels {
		w := 1.0
		if i == peak {
			w = 3 + float64(seed%5)
		}
		out[l] = w
		total += w
	}
	for l := range out {
		out[l] /= total
	}
	return out
}

func argmax(p map[string]float64) string {
	var best string
	for k, v := range p {
		if best == "" || v > p[best] || (v == p[best] && k < best) {
			best = k
		}
	}
	return best
}
