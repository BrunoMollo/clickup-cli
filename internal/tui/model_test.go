package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"clickdown/internal/sprint"
	"clickdown/internal/tasks"
)

type fakeLoader struct {
	snapshot tasks.Snapshot
	err      error
}

func (f fakeLoader) Load(context.Context) (tasks.Snapshot, error) { return f.snapshot, f.err }

type fakeOpener struct {
	urls []string
}

type sequenceLoader struct {
	snapshots []tasks.Snapshot
	calls     int
}

func (l *sequenceLoader) Load(context.Context) (tasks.Snapshot, error) {
	index := l.calls
	if index >= len(l.snapshots) {
		index = len(l.snapshots) - 1
	}
	l.calls++
	return l.snapshots[index], nil
}

type changingRefresher struct {
	calls int
}

func (r *changingRefresher) RefreshActive(context.Context) (bool, error) {
	r.calls++
	return true, nil
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
	m := NewModel(fakeLoader{}, opener, false, nil)
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
	m := NewModel(fakeLoader{}, &fakeOpener{}, false, nil)
	m.Update(loadMsg{snapshot: testSnapshot()})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.selectedID != "parent" {
		t.Fatalf("selección=%q", m.selectedID)
	}
}

func TestTaskMetadataPresentation(t *testing.T) {
	first := &tasks.Task{
		ID:          "first",
		Name:        "Primera",
		Status:      tasks.Status{Name: "En curso"},
		Priority:    &tasks.Priority{Name: "urgent", Color: "#F50000"},
		Assignees:   []string{"Bruno Mollo"},
		SprintNames: []string{"Sprint Julio"},
	}
	second := &tasks.Task{
		ID:          "second",
		Name:        "Segunda",
		Status:      tasks.Status{Name: "En curso"},
		Priority:    &tasks.Priority{Name: "high", Color: "#FF8C00"},
		Assignees:   []string{"Otra Persona"},
		SprintNames: []string{"Sprint Agosto"},
	}
	snapshot := tasks.Snapshot{
		Sprints: []sprint.Sprint{{Name: "Sprint Julio"}},
		Forest:  tasks.BuildForest([]*tasks.Task{first, second}),
	}
	m := NewModel(fakeLoader{}, &fakeOpener{}, false, nil)
	m.width = 80
	m.Update(loadMsg{snapshot: snapshot})

	view := m.View()
	if strings.Contains(view, "Sprint Julio") || strings.Contains(view, "Sprint Agosto") {
		t.Fatalf("sprint visible: %q", view)
	}
	if !strings.Contains(view, "Bruno Mollo") || strings.Contains(view, "Otra Persona") {
		t.Fatalf("responsables visibles incorrectos: %q", view)
	}

	m.Update(tea.MouseMsg{X: 1, Y: 5, Action: tea.MouseActionMotion})
	view = m.View()
	if m.selectedID != "second" || strings.Contains(view, "Bruno Mollo") || !strings.Contains(view, "Otra Persona") {
		t.Fatalf("hover incorrecto: selected=%q view=%q", m.selectedID, view)
	}

	var secondLine displayLine
	for _, line := range m.lines {
		if line.kind == taskLine && line.node.Task.ID == "second" {
			secondLine = line
			break
		}
	}
	rendered := renderLine(m, secondLine, 80)
	if lipgloss.Width(rendered) != 80 {
		t.Fatalf("ancho=%d; metadata no alineada", lipgloss.Width(rendered))
	}
}

func TestPriorityColorFallbacks(t *testing.T) {
	if priorityColor("urgent", "") != lipgloss.Color("#F50000") {
		t.Fatal("urgent sin rojo")
	}
	if priorityColor("low", "#123456") != lipgloss.Color("#123456") {
		t.Fatal("color API no respetado")
	}
}

func TestCacheWatchEmitsOneSnapshotAfterVisibleChange(t *testing.T) {
	oldSnapshot := tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{{ID: "1", Name: "Old"}})}
	newSnapshot := tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{{ID: "1", Name: "New"}})}
	loader := &sequenceLoader{snapshots: []tasks.Snapshot{oldSnapshot, newSnapshot}}
	refresher := &changingRefresher{}
	m := NewModel(loader, &fakeOpener{}, false, refresher)
	defer m.cancel()

	initial := m.Init()()
	_, watch := m.Update(initial)
	if watch == nil {
		t.Fatal("watch no iniciado")
	}
	message := watch()
	changed, ok := message.(cacheSnapshotMsg)
	if !ok || changed.snapshot.Forest[0].Task.Name != "New" || refresher.calls != 1 {
		t.Fatalf("message=%T calls=%d", message, refresher.calls)
	}
}

func TestCacheWatchSkipsUnchangedSnapshot(t *testing.T) {
	oldSnapshot := tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{{ID: "1", Name: "Old"}})}
	newSnapshot := tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{{ID: "1", Name: "New"}})}
	loader := &sequenceLoader{snapshots: []tasks.Snapshot{oldSnapshot, oldSnapshot, newSnapshot}}
	refresher := &changingRefresher{}
	m := NewModel(loader, &fakeOpener{}, false, refresher)
	m.refreshEvery = time.Millisecond
	defer m.cancel()

	initial := m.Init()()
	_, watch := m.Update(initial)
	message := watch()
	if _, ok := message.(cacheSnapshotMsg); !ok || refresher.calls != 2 {
		t.Fatalf("message=%T calls=%d; emitió antes de cambio visible", message, refresher.calls)
	}
}

func TestSnapshotSignatureIgnoresRefreshTimestamp(t *testing.T) {
	snapshot := tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{{ID: "1", Name: "Task"}}), LoadedAt: time.Unix(1, 0)}
	other := snapshot
	other.LoadedAt = time.Unix(2, 0)
	if snapshotSignature(snapshot) != snapshotSignature(other) {
		t.Fatal("timestamp causó render innecesario")
	}
	changed := tasks.Snapshot{Forest: tasks.BuildForest([]*tasks.Task{{ID: "1", Name: "Changed"}})}
	if snapshotSignature(snapshot) == snapshotSignature(changed) {
		t.Fatal("cambio visible no detectado")
	}
}
