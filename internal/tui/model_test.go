package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"botty/internal/tasks"
)

type fakeLoader struct {
	snapshot tasks.Snapshot
	err      error
}

func (f fakeLoader) Load(context.Context) (tasks.Snapshot, error) { return f.snapshot, f.err }

type fakeOpener struct {
	urls []string
}

func (o *fakeOpener) Open(url string) error {
	o.urls = append(o.urls, url)
	return nil
}

func testSnapshot() tasks.Snapshot {
	openParent := &tasks.Task{ID: "parent", Name: "Parent", URL: "https://app.clickup.com/t/parent", Status: tasks.Status{Name: "Doing", Type: "custom"}}
	openChild := &tasks.Task{ID: "child", ParentID: "parent", Name: "Child", URL: "https://app.clickup.com/t/child", Status: tasks.Status{Name: "Review", Type: "custom"}}
	closed := &tasks.Task{ID: "closed", Name: "Closed", URL: "https://app.clickup.com/t/closed", Status: tasks.Status{Name: "Done", Type: "closed"}}
	return tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{openParent, openChild, closed})}
}

func TestModelExpandsTogglesAndOpens(t *testing.T) {
	opener := &fakeOpener{}
	m := NewModel(fakeLoader{}, opener, false)
	m.Update(loadMsg{snapshot: testSnapshot()})
	if m.nodes["closed"] != nil || m.nodes["child"] != nil {
		t.Fatal("estado inicial debe ocultar cerradas y subtareas colapsadas")
	}

	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.nodes["child"] == nil || !m.expanded["parent"] {
		t.Fatal("subtarea no expandida")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedID != "child" {
		t.Fatalf("selección=%q", m.selectedID)
	}
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("Enter no produjo comando")
	}
	m.Update(command())
	if len(opener.urls) != 1 || opener.urls[0] != "https://app.clickup.com/t/child" {
		t.Fatalf("URLs=%v", opener.urls)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.nodes["closed"] == nil {
		t.Fatal("modo completo no mostró cerrada")
	}
}

func TestModelLeftReturnsToParent(t *testing.T) {
	m := NewModel(fakeLoader{}, &fakeOpener{}, false)
	m.Update(loadMsg{snapshot: testSnapshot()})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.selectedID != "parent" {
		t.Fatalf("selección=%q", m.selectedID)
	}
}
