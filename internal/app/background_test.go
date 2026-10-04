package app

import (
	"errors"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// withModal opens each kind of modal that captures all messages.
var withModal = map[string]func(*Model){
	"palette":      func(m *Model) { m.showPalette = true },
	"create form":  func(m *Model) { m.creating = true },
	"quick action": func(m *Model) { m.qaMode = "comment" },
	"nudge":        func(m *Model) { m.nudging = true },
	"dup dialog":   func(m *Model) { m.dupDialogOpen = true },
}

func TestUpdateBackgroundWhileModalOpen(t *testing.T) {
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	for name, open := range withModal {
		t.Run(name, func(t *testing.T) {
			m := newJSONLRefreshModel(t, issues)
			m.requestRefresh()
			open(&m)

			// The fetch lands while the modal has the keyboard.
			model, cmd := m.Update(refreshResultMsg{gen: m.refresh.fetchGen, msg: data.FileUnchangedMsg{}})
			got := model.(Model)
			if got.refresh.inFlight || cmd == nil {
				t.Fatalf("inFlight %v: a result swallowed by the modal would stop reloads for good", got.refresh.inFlight)
			}

			// And the next tick still starts a fetch.
			model, cmd = got.Update(refreshTickMsg{gen: got.refresh.tickGen})
			if cmd == nil || !model.(Model).refresh.inFlight {
				t.Fatal("expected the refresh timer to keep working behind the modal")
			}
		})
	}
}

func TestUpdateBackgroundTownStatusWhileModalOpen(t *testing.T) {
	m := New(nil, data.Source{}, data.DefaultBlockingTypes)
	m.gtPollInFlight = true
	m.showPalette = true
	model, _ := m.Update(townStatusMsg{err: errors.New("gt status: timeout")})
	if model.(Model).gtPollInFlight {
		t.Fatal("a status poll swallowed by the palette would block every later poll")
	}
}

func TestUpdateBackgroundToastWhileModalOpen(t *testing.T) {
	m := New(nil, data.Source{}, data.DefaultBlockingTypes)
	m.toast, _ = components.ShowToast("hello", components.ToastInfo, toastDuration)
	m.creating = true
	model, _ := m.Update(components.ToastDismissMsg{})
	if model.(Model).toast.Active() {
		t.Fatal("expected the toast to be dismissed behind the form")
	}
}
