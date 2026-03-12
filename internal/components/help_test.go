package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHelpViewRendersContent(t *testing.T) {
	h := NewHelp(120, 200)
	view := h.View()
	// Check for content that appears in the help overlay
	if !strings.Contains(view, "Quit application") {
		t.Fatal("should show 'Quit application' binding")
	}
	if !strings.Contains(view, "close") {
		t.Fatal("should show close hint")
	}
}

func TestHelpPagination(t *testing.T) {
	// Small height forces pagination
	h := NewHelp(80, 30)
	pages := h.pageCount()
	if pages < 2 {
		t.Fatalf("expected at least 2 pages at height 30, got %d", pages)
	}
}

func TestHelpPageNavigation(t *testing.T) {
	h := NewHelp(80, 30)
	if h.pageCount() < 2 {
		t.Skip("pagination not triggered at this height")
	}

	// Navigate right
	h, handled := h.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if !handled {
		t.Fatal("expected l to be handled")
	}
	if h.page != 1 {
		t.Fatalf("after l, page = %d, want 1", h.page)
	}

	// Navigate left
	h, handled = h.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if !handled {
		t.Fatal("expected h to be handled")
	}
	if h.page != 0 {
		t.Fatalf("after h, page = %d, want 0", h.page)
	}

	// Can't go below 0
	h, _ = h.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if h.page != 0 {
		t.Fatalf("page should clamp at 0, got %d", h.page)
	}
}

func TestHelpPageNavigationArrowKeys(t *testing.T) {
	h := NewHelp(80, 30)
	if h.pageCount() < 2 {
		t.Skip("pagination not triggered at this height")
	}

	h, handled := h.Update(tea.KeyPressMsg{Code: tea.KeyRight, Text: "right"})
	if !handled {
		t.Fatal("expected right arrow to be handled")
	}
	if h.page != 1 {
		t.Fatalf("after right, page = %d, want 1", h.page)
	}

	h, handled = h.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Text: "left"})
	if !handled {
		t.Fatal("expected left arrow to be handled")
	}
	if h.page != 0 {
		t.Fatalf("after left, page = %d, want 0", h.page)
	}
}

func TestHelpUnhandledKey(t *testing.T) {
	h := NewHelp(80, 30)
	_, handled := h.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if handled {
		t.Fatal("z should not be handled")
	}
}

func TestHelpSinglePageNoIndicator(t *testing.T) {
	// Very tall terminal — all sections fit on one page
	h := NewHelp(120, 200)
	if h.pageCount() != 1 {
		t.Skip("pagination still triggered at height 200")
	}
	view := h.View()
	if strings.Contains(view, "Page") {
		t.Fatal("should not show page indicator when only 1 page")
	}
}

func TestHelpMultiPageShowsIndicator(t *testing.T) {
	h := NewHelp(80, 30)
	if h.pageCount() < 2 {
		t.Skip("pagination not triggered at this height")
	}
	view := h.View()
	if !strings.Contains(view, "Page 1/") {
		t.Fatal("should show page indicator when multiple pages")
	}
}

func TestHelpProblemsSection(t *testing.T) {
	// Verify the PROBLEMS section exists in the help sections
	sections := allSections()
	found := false
	for _, s := range sections {
		if !strings.Contains(s.title, "PROBLEMS") {
			continue
		}
		found = true
		// Verify R key is documented
		hasR := false
		for _, b := range s.bindings {
			if b.key == "R" {
				hasR = true
				break
			}
		}
		if !hasR {
			t.Fatal("PROBLEMS section should have R keybinding")
		}
		break
	}
	if !found {
		t.Fatal("should have a PROBLEMS section in help")
	}
}

func TestPaginateSectionsEmptyMaxLines(t *testing.T) {
	sections := allSections()
	pages := paginateSections(sections, 0)
	if len(pages) != 1 {
		t.Fatalf("expected 1 page with 0 maxLines, got %d", len(pages))
	}
}
