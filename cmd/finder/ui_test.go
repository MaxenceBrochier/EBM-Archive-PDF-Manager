package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ebmarchive/internal/manifest"
)

func uiTestCatalog(n int) *catalog {
	fields := []manifest.Field{{Name: "lot", Label: "Lot"}}
	entries := bigSet(n)
	valEntries := entries
	if n > 1 {
		valEntries = entries[1:]
	}
	return &catalog{
		idx: &manifest.Index{Fields: fields},
		tabs: []tab{
			{label: "Tous", entries: entries},
			{name: "VAL", label: "VAL", entries: valEntries},
		},
	}
}

func updateTestModel(m model, msg tea.Msg) model {
	updated, _ := m.Update(msg)
	return updated.(model)
}

func TestUpdateLoadsCatalogAndNavigatesTabs(t *testing.T) {
	m := newModel(t.TempDir())
	m = updateTestModel(m, loadedMsg{cat: uiTestCatalog(3)})
	if m.loading || len(m.results) != 3 || m.active != 0 {
		t.Fatalf("loaded model = loading %t, %d results, active tab %d", m.loading, len(m.results), m.active)
	}

	m = updateTestModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", m.cursor)
	}
	m = updateTestModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.active != 1 || len(m.results) != 2 {
		t.Fatalf("model after tab = active %d, %d results, want active 1 and 2 results", m.active, len(m.results))
	}
	m = updateTestModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.active != 0 {
		t.Fatalf("tab navigation did not wrap: active = %d", m.active)
	}
}

func TestUpdateSearchAndClearStatus(t *testing.T) {
	m := newModel(t.TempDir())
	m = updateTestModel(m, loadedMsg{cat: uiTestCatalog(3)})
	m = updateTestModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L24A000002")})
	if len(m.results) != 1 || m.results[0].cells[0] != "L24A000002" {
		t.Fatalf("search results = %#v, want the matching lot only", m.results)
	}

	m = updateTestModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.input.Value() != "" || len(m.results) != 3 {
		t.Fatalf("escape did not clear search: query %q, %d results", m.input.Value(), len(m.results))
	}

	m.status, m.statusID = "latest", 2
	m = updateTestModel(m, clearStatusMsg(1))
	if m.status != "latest" {
		t.Fatalf("stale clear removed current status %q", m.status)
	}
	m = updateTestModel(m, clearStatusMsg(2))
	if m.status != "" {
		t.Fatalf("current clear left status %q", m.status)
	}
}

func TestUpdateWindowSizeAndScrollBoundaries(t *testing.T) {
	m := newModel(t.TempDir())
	m = updateTestModel(m, loadedMsg{cat: uiTestCatalog(100)})
	m = updateTestModel(m, tea.WindowSizeMsg{Width: 120, Height: 16})
	if m.w != 120 || m.h != 16 || m.input.Width != 90 {
		t.Fatalf("window dimensions = %dx%d, input width %d", m.w, m.h, m.input.Width)
	}

	m.move(10)
	if m.cursor != 10 || m.offset == 0 {
		t.Fatalf("after moving down: cursor %d, offset %d", m.cursor, m.offset)
	}
	m.move(-100)
	if m.cursor != 0 || m.offset != 0 {
		t.Fatalf("after moving above first result: cursor %d, offset %d", m.cursor, m.offset)
	}
}
