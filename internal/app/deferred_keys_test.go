package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

func setupDeferredKeyModel(t *testing.T) (model Model, filter func(tea.Model, tea.Msg) tea.Msg) {
	t.Helper()

	guard := NewOSCGuard()
	filter = guard.Filter()
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
	}
	m := NewWithGuard(issues, data.Source{}, data.DefaultBlockingTypes, guard, false)
	m.startedAt = time.Now().Add(-time.Second)

	readyModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return readyModel.(Model), filter
}

func sendFiltered(t *testing.T, m Model, filter func(tea.Model, tea.Msg) tea.Msg, msg tea.Msg) (Model, tea.Cmd, bool) {
	t.Helper()

	filtered := filter(nil, msg)
	if filtered == nil {
		return m, nil, false
	}

	model, cmd := m.Update(filtered)
	return model.(Model), cmd, true
}

func TestDeferredKeyPassesAfterDelay(t *testing.T) {
	m, filter := setupDeferredKeyModel(t)

	var cmd tea.Cmd
	var ok bool
	m, cmd, ok = sendFiltered(t, m, filter, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if !ok {
		t.Fatal("expected q to pass filter")
	}
	if cmd == nil {
		t.Fatal("expected deferred command after staging q")
	}

	msg := cmd()
	deferred, ok := msg.(deferredKeyMsg)
	if !ok {
		t.Fatalf("expected deferredKeyMsg, got %T", msg)
	}

	model, quitCmd := m.Update(deferred)
	m = model.(Model)
	if quitCmd == nil {
		t.Fatal("expected q to produce a quit command after delay")
	}
	quitMsg := quitCmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", quitMsg)
	}
	if len(m.pendingKeys) != 0 {
		t.Fatal("expected pending key queue to be cleared after deferred delivery")
	}
}

func TestDeferredKeyDropsSuspiciousPair(t *testing.T) {
	m, filter := setupDeferredKeyModel(t)

	m, firstCmd, ok := sendFiltered(t, m, filter, tea.KeyPressMsg{Code: '1', Text: "1"})
	if !ok {
		t.Fatal("expected 1 to pass filter")
	}
	if firstCmd == nil {
		t.Fatal("expected deferred command after staging 1")
	}

	time.Sleep(20 * time.Millisecond)

	m, secondCmd, ok := sendFiltered(t, m, filter, tea.KeyPressMsg{Code: ';', Text: ";"})
	if !ok {
		t.Fatal("expected ; to reach Update for pair detection")
	}
	if secondCmd != nil {
		t.Fatal("expected suspicious pair to be dropped without routing a command")
	}
	if len(m.pendingKeys) != 0 {
		t.Fatal("expected pending key queue to be cleared after suspicious pair drop")
	}

	model, cmd := m.Update(firstCmd())
	m = model.(Model)
	if cmd != nil {
		t.Fatal("expected stale deferred message to be ignored after pair drop")
	}
	if len(m.pendingKeys) != 0 {
		t.Fatal("expected no pending key after stale deferred message")
	}
}

func TestDeferredKeyDropsAfterFilterSuppression(t *testing.T) {
	m, filter := setupDeferredKeyModel(t)

	m, firstCmd, ok := sendFiltered(t, m, filter, tea.KeyPressMsg{Code: ']', Text: "]"})
	if !ok {
		t.Fatal("expected ] to pass filter and stage")
	}
	if firstCmd == nil {
		t.Fatal("expected deferred command after staging ]")
	}

	if filtered := filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"}); filtered != nil {
		t.Fatal("expected 1 to be suppressed by the shared guard filter")
	}

	model, cmd := m.Update(firstCmd())
	m = model.(Model)
	if cmd != nil {
		t.Fatal("expected deferred ] to be dropped after later filter suppression")
	}
	if len(m.pendingKeys) != 0 {
		t.Fatal("expected pending key queue to be cleared after deferred drop")
	}
}

func TestDeferredQuickActionDigitWaitsForTimer(t *testing.T) {
	m, filter := setupDeferredKeyModel(t)

	m, firstCmd, ok := sendFiltered(t, m, filter, tea.KeyPressMsg{Code: '3', Text: "3"})
	if !ok {
		t.Fatal("expected 3 to pass filter and stage")
	}
	if firstCmd == nil {
		t.Fatal("expected deferred command after staging 3")
	}
	if len(m.pendingKeys) != 1 {
		t.Fatalf("expected 1 pending key after staging 3, got %d", len(m.pendingKeys))
	}

	time.Sleep(20 * time.Millisecond)

	m, secondCmd, ok := sendFiltered(t, m, filter, tea.KeyPressMsg{Code: '2', Text: "2"})
	if !ok {
		t.Fatal("expected 2 to pass filter and stage")
	}
	if secondCmd == nil {
		t.Fatal("expected second deferred command after staging 2 behind 3")
	}
	if len(m.pendingKeys) != 2 {
		t.Fatalf("expected 2 pending keys after staging 3 and 2, got %d", len(m.pendingKeys))
	}

	if filtered := filter(nil, uv.UnknownEvent("\x1b]11;rgb:1f1f/2323/3535")); filtered != nil {
		t.Fatal("expected UnknownEvent to be dropped by shared guard")
	}

	model, cmd := m.Update(firstCmd())
	m = model.(Model)
	if cmd != nil {
		t.Fatal("expected deferred 3 to be dropped after later suspicious input")
	}
	if len(m.pendingKeys) != 1 {
		t.Fatalf("expected only the second pending key to remain, got %d", len(m.pendingKeys))
	}

	model, cmd = m.Update(secondCmd())
	m = model.(Model)
	if cmd != nil {
		t.Fatal("expected deferred 2 to be dropped after later suspicious input")
	}
	if len(m.pendingKeys) != 0 {
		t.Fatal("expected pending key queue to be cleared after both deferred drops")
	}
}
