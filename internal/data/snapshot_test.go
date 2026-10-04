package data

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var snapNow = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func snapIssue() Issue {
	due := snapNow.Add(36 * time.Hour)
	return Issue{
		ID:                 "mg-42",
		Title:              "Login loop on Safari after 2FA",
		Description:        "Users bounce back to login.\n```\nGET /auth 302\n```\nSeen since v1.2.",
		Status:             StatusOpen,
		Priority:           PriorityHigh,
		IssueType:          TypeBug,
		Owner:              "alice@example.com",
		Assignee:           "bob@example.com",
		CreatedBy:          "alice@example.com",
		CreatedAt:          snapNow.Add(-10 * 24 * time.Hour),
		UpdatedAt:          snapNow.Add(-60 * time.Hour), // 2 days, with room to bump within the day
		Notes:              "api_key=sk-notes-secret-value-12345",
		AcceptanceCriteria: "no redirect loop",
		Labels:             []string{"ui", "auth"},
		DueAt:              &due,
		Metadata:           map[string]interface{}{"host": "db.internal"},
		CommentCount:       3,
		Dependencies:       []Dependency{{Type: "blocks", DependsOnID: "mg-7"}, {Type: "blocks", DependsOnID: "gone-1"}},
	}
}

func snapEval() DepEval {
	return DepEval{BlockingIDs: []string{"mg-7"}, MissingIDs: []string{"gone-1"}, IsBlocked: true}
}

func TestSnapshotForJudge(t *testing.T) {
	s := SnapshotForJudge(snapIssue(), snapEval(), SnapshotStandard, snapNow)

	if s.ID != "mg-42" || s.Title != "Login loop on Safari after 2FA" || s.Type != "bug" || s.Status != "open" || s.Priority != 1 {
		t.Fatalf("identity fields: %+v", s)
	}
	if s.AgeDays != 10 || s.SinceUpdateDays != 2 {
		t.Fatalf("ages: %+v", s)
	}
	if s.DueInDays == nil || *s.DueInDays != 1 {
		t.Fatalf("due: %v", s.DueInDays)
	}
	if s.DeferredForDays != nil {
		t.Fatalf("deferred should be nil: %v", *s.DeferredForDays)
	}
	if s.Blocking != 1 || s.MissingDeps != 1 || s.CommentCount != 3 {
		t.Fatalf("counts: %+v", s)
	}
	if strings.Join(s.Labels, ",") != "auth,ui" {
		t.Fatalf("labels should be sorted: %v", s.Labels)
	}
	if !strings.Contains(s.Description, "[code]") || strings.Contains(s.Description, "GET /auth") {
		t.Fatalf("code fence not stripped: %q", s.Description)
	}
	if !strings.Contains(s.Description, "Seen since v1.2.") {
		t.Fatalf("text after the fence lost: %q", s.Description)
	}
}

// TestSnapshotForJudgeEdgeCaseNeverSendsPeopleOrNotes pins the field list:
// the JSON that leaves the machine must not carry these, whatever the scope.
func TestSnapshotForJudgeEdgeCaseNeverSendsPeopleOrNotes(t *testing.T) {
	for _, scope := range []SnapshotScope{SnapshotMinimal, SnapshotStandard} {
		b, err := json.Marshal(SnapshotForJudge(snapIssue(), snapEval(), scope, snapNow))
		if err != nil {
			t.Fatal(err)
		}
		js := string(b)
		for _, leak := range []string{"alice", "bob", "example.com", "sk-notes", "no redirect loop", "db.internal", "mg-7", "gone-1", "2026-"} {
			if strings.Contains(js, leak) {
				t.Errorf("scope %d: snapshot carries %q:\n%s", scope, leak, js)
			}
		}
	}
}

func TestSnapshotForJudgeEdgeCaseMinimalScope(t *testing.T) {
	s := SnapshotForJudge(snapIssue(), snapEval(), SnapshotMinimal, snapNow)
	if s.Description != "" {
		t.Fatalf("minimal scope sent a description: %q", s.Description)
	}
}

func TestSnapshotForJudgeEdgeCaseDescriptionTruncated(t *testing.T) {
	iss := snapIssue()
	iss.Description = strings.Repeat("word ", 300) // 1500 runes
	s := SnapshotForJudge(iss, DepEval{}, SnapshotStandard, snapNow)
	if n := len([]rune(s.Description)); n > snapshotDescriptionRunes+1 || !strings.HasSuffix(s.Description, "…") {
		t.Fatalf("description %d runes, suffix %q", n, s.Description[len(s.Description)-3:])
	}
}

func TestSnapshotForJudgeEdgeCaseDeferredAndOverdue(t *testing.T) {
	iss := snapIssue()
	due := snapNow.Add(-50 * time.Hour)
	defer_ := snapNow.Add(72 * time.Hour)
	iss.DueAt, iss.DeferUntil = &due, &defer_
	s := SnapshotForJudge(iss, DepEval{}, SnapshotMinimal, snapNow)
	if s.DueInDays == nil || *s.DueInDays != -2 {
		t.Fatalf("overdue by 50h should be -2 days: %v", s.DueInDays)
	}
	if s.DeferredForDays == nil || *s.DeferredForDays != 3 {
		t.Fatalf("deferred 72h should be 3 days: %v", s.DeferredForDays)
	}
	past := snapNow.Add(-time.Hour)
	iss.DeferUntil = &past
	if s := SnapshotForJudge(iss, DepEval{}, SnapshotMinimal, snapNow); s.DeferredForDays != nil {
		t.Fatal("an elapsed defer_until is not a deferral")
	}
}

func TestSnapshotForJudgeEdgeCaseZeroUpdatedAt(t *testing.T) {
	iss := snapIssue()
	iss.UpdatedAt = time.Time{}
	s := SnapshotForJudge(iss, DepEval{}, SnapshotMinimal, snapNow)
	if s.SinceUpdateDays != s.AgeDays {
		t.Fatalf("missing updated_at should fall back to created_at: %+v", s)
	}
}

func TestIssueSnapshotHash(t *testing.T) {
	base := SnapshotForJudge(snapIssue(), snapEval(), SnapshotStandard, snapNow).Hash()
	if len(base) != 32 {
		t.Fatalf("hash %q", base)
	}

	// Churn that does not change what is sent keeps the hash: an updated_at
	// bump within the same day, a label reorder, an assignee change.
	same := snapIssue()
	same.UpdatedAt = same.UpdatedAt.Add(5 * time.Hour)
	same.Labels = []string{"auth", "ui"}
	same.Assignee = "carol"
	if got := SnapshotForJudge(same, snapEval(), SnapshotStandard, snapNow).Hash(); got != base {
		t.Fatal("hash changed on churn the snapshot does not carry")
	}

	// A new label, a priority change or a day passing changes it.
	labelled := snapIssue()
	labelled.Labels = append(labelled.Labels, "security")
	if SnapshotForJudge(labelled, snapEval(), SnapshotStandard, snapNow).Hash() == base {
		t.Fatal("hash unchanged after a label was added")
	}
	if SnapshotForJudge(snapIssue(), snapEval(), SnapshotStandard, snapNow.Add(24*time.Hour)).Hash() == base {
		t.Fatal("hash unchanged a day later; time-dependent questions would never be re-asked")
	}
	if SnapshotForJudge(snapIssue(), snapEval(), SnapshotMinimal, snapNow).Hash() == base {
		t.Fatal("minimal and standard snapshots must hash differently")
	}
}

func TestParseSnapshotScope(t *testing.T) {
	cases := []struct {
		in   string
		want SnapshotScope
	}{
		{"", SnapshotStandard},
		{"standard", SnapshotStandard},
		{" Minimal ", SnapshotMinimal}, // trimmed and case-folded
	}
	for _, c := range cases {
		got, ok := ParseSnapshotScope(c.in)
		if !ok || got != c.want {
			t.Errorf("ParseSnapshotScope(%q) = %v, %v", c.in, got, ok)
		}
	}
	if got, ok := ParseSnapshotScope("rich"); ok || got != SnapshotStandard {
		t.Errorf("unknown scope: %v, %v (want default, false)", got, ok)
	}
}

func TestScrubSecrets(t *testing.T) {
	cases := map[string]string{
		"":                                 "",
		"plain title about auth":           "plain title about auth",
		"key sk-abc123DEF456ghi789 leaked": "key [redacted] leaked",
		"token ghp_abcdefghijklmnopqrstuvwxyz0123":                  "token [redacted]",
		"export API_KEY=supersecret123":                             "export API_KEY=[redacted]",
		"password: hunter2 was set":                                 "password: [redacted] was set",
		`Authorization: "Bearer abcdefghijklmnopqrstuv"`:            "Authorization: [redacted]",
		"see commit 3f2a9c1e7b4d5a6f8e9d0c1b2a3f4e5d6c7b8a9f":       "see commit [redacted]",
		"AKIAIOSFODNN7EXAMPLE in the logs":                          "[redacted] in the logs",
		"xoxb-123456789012-abcdefghijkl":                            "[redacted]",
		"path /a/very/long/path/without/any/digits/in/it/at/all/ok": "path /a/very/long/path/without/any/digits/in/it/at/all/ok",
		"blob dGhpcyBpcyBhIGxvbmcgYmFzZTY0IHRva2VuIDEyMzQ1Njc4OTA=": "blob [redacted]",
		"a token budget of 400 is fine":                             "a token budget of 400 is fine",
	}
	for in, want := range cases {
		if got := ScrubSecrets(in); got != want {
			t.Errorf("ScrubSecrets(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
