package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"clickdown/internal/browser"
	"clickdown/internal/tasks"
)

type loadMsg struct {
	snapshot tasks.Snapshot
	err      error
}

type openMsg struct {
	err error
}

type cacheSnapshotMsg struct {
	snapshot tasks.Snapshot
}

type manualRefreshMsg struct {
	snapshot tasks.Snapshot
	changed  bool
	err      error
}

type watchStoppedMsg struct{}

type CacheRefresher interface {
	RefreshActive(ctx context.Context) (bool, error)
}

type lineKind int

const (
	groupLine lineKind = iota
	taskLine
)

type displayLine struct {
	kind     lineKind
	group    tasks.StatusGroup
	node     *tasks.Node
	depth    int
	parentID string
}

type Model struct {
	loader        tasks.Loader
	opener        browser.Opener
	refresher     CacheRefresher
	includeClosed bool
	snapshot      tasks.Snapshot
	lines         []displayLine
	nodes         map[string]*tasks.Node
	parents       map[string]string
	expanded      map[string]bool
	selectedID    string
	offset        int
	width         int
	height        int
	loading       bool
	err           error
	notice        string
	ctx           context.Context
	cancel        context.CancelFunc
	watchCancel   context.CancelFunc
	refreshEvery  time.Duration
}

func NewModel(loader tasks.Loader, opener browser.Opener, includeClosed bool, refresher CacheRefresher) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		loader:        loader,
		opener:        opener,
		refresher:     refresher,
		includeClosed: includeClosed,
		expanded:      make(map[string]bool),
		nodes:         make(map[string]*tasks.Node),
		parents:       make(map[string]string),
		width:         100,
		height:        30,
		loading:       true,
		ctx:           ctx,
		cancel:        cancel,
		refreshEvery:  20 * time.Second,
	}
}

func (m *Model) Init() tea.Cmd {
	return m.loadCmd()
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		m.ensureVisible()
		return m, nil
	case loadMsg:
		m.loading = false
		m.err = message.err
		m.notice = ""
		if message.err == nil {
			m.snapshot = message.snapshot
			m.rebuild()
			return m, m.startCacheWatch(true)
		}
		return m, nil
	case cacheSnapshotMsg:
		m.snapshot = message.snapshot
		m.err = nil
		m.notice = ""
		m.rebuild()
		return m, m.startCacheWatch(false)
	case manualRefreshMsg:
		if message.changed {
			m.snapshot = message.snapshot
			m.err = nil
			m.rebuild()
		}
		if message.err != nil {
			m.notice = "Cache parcial: " + message.err.Error()
		} else if message.changed {
			m.notice = "Datos actualizados"
		} else {
			m.notice = "Sin cambios"
		}
		return m, m.startCacheWatch(false)
	case watchStoppedMsg:
		return m, nil
	case openMsg:
		if message.err != nil {
			m.notice = message.err.Error()
		} else {
			m.notice = "Tarea abierta en navegador"
		}
		return m, nil
	case tea.MouseMsg:
		return m.handleMouse(tea.MouseEvent(message))
	case tea.KeyMsg:
		return m.handleKey(message)
	}
	return m, nil
}

func (m *Model) handleMouse(mouse tea.MouseEvent) (tea.Model, tea.Cmd) {
	if mouse.Button == tea.MouseButtonWheelUp {
		m.move(-3)
		return m, nil
	}
	if mouse.Button == tea.MouseButtonWheelDown {
		m.move(3)
		return m, nil
	}
	if mouse.Action != tea.MouseActionMotion && mouse.Action != tea.MouseActionPress {
		return m, nil
	}
	lineIndex := mouse.Y - 3 + m.offset
	if lineIndex < 0 || lineIndex >= len(m.lines) {
		return m, nil
	}
	line := m.lines[lineIndex]
	if line.kind == taskLine {
		m.selectedID = line.node.Task.ID
		m.ensureVisible()
	}
	return m, nil
}

func (m *Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "r":
		if !m.loading && m.refresher != nil {
			m.stopCacheWatch()
			m.notice = "Actualizando cache…"
			return m, m.manualRefreshCmd()
		}
	}
	if m.loading || m.err != nil {
		return m, nil
	}

	switch key.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.move(-m.contentHeight())
	case "pgdown":
		m.move(m.contentHeight())
	case "a":
		m.includeClosed = !m.includeClosed
		m.notice = ""
		m.rebuild()
	case " ", "right", "l":
		if node := m.nodes[m.selectedID]; node != nil && len(node.Children) > 0 {
			if key.String() == " " {
				m.expanded[m.selectedID] = !m.expanded[m.selectedID]
			} else {
				m.expanded[m.selectedID] = true
			}
			m.rebuild()
		}
	case "left", "h":
		if m.expanded[m.selectedID] {
			m.expanded[m.selectedID] = false
			m.rebuild()
		} else if parent := m.parents[m.selectedID]; parent != "" {
			m.selectedID = parent
			m.ensureVisible()
		}
	case "enter":
		if node := m.nodes[m.selectedID]; node != nil {
			return m, m.openCmd(node.Task.URL)
		}
	}
	return m, nil
}

func (m *Model) View() string {
	return render(m)
}

func (m *Model) loadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 60*time.Second)
		defer cancel()
		snapshot, err := m.loader.Load(ctx)
		return loadMsg{snapshot: snapshot, err: err}
	}
}

func (m *Model) openCmd(taskURL string) tea.Cmd {
	return func() tea.Msg {
		return openMsg{err: m.opener.Open(taskURL)}
	}
}

func (m *Model) startCacheWatch(immediate bool) tea.Cmd {
	if m.refresher == nil {
		return nil
	}
	m.stopCacheWatch()
	ctx, cancel := context.WithCancel(m.ctx)
	m.watchCancel = cancel
	signature := snapshotSignature(m.snapshot)
	return func() tea.Msg {
		dirty := false
		if !immediate && !waitContext(ctx, m.refreshEvery) {
			return watchStoppedMsg{}
		}
		for {
			changed, _ := m.refresher.RefreshActive(ctx)
			dirty = dirty || changed
			if ctx.Err() != nil {
				return watchStoppedMsg{}
			}
			if dirty {
				snapshot, err := m.loader.Load(ctx)
				if err == nil {
					dirty = false
					if snapshotSignature(snapshot) != signature {
						return cacheSnapshotMsg{snapshot: snapshot}
					}
				}
			}
			if !waitContext(ctx, m.refreshEvery) {
				return watchStoppedMsg{}
			}
		}
	}
}

func (m *Model) stopCacheWatch() {
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
}

func (m *Model) manualRefreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 60*time.Second)
		defer cancel()
		changed, refreshErr := m.refresher.RefreshActive(ctx)
		if !changed {
			return manualRefreshMsg{err: refreshErr}
		}
		snapshot, loadErr := m.loader.Load(ctx)
		return manualRefreshMsg{
			snapshot: snapshot,
			changed:  loadErr == nil && snapshotSignature(snapshot) != snapshotSignature(m.snapshot),
			err:      errorsJoin(refreshErr, loadErr),
		}
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func snapshotSignature(snapshot tasks.Snapshot) string {
	hash := sha256.New()
	for _, warning := range snapshot.Warnings {
		_, _ = io.WriteString(hash, "warning\x00"+warning.Error()+"\x00")
	}
	var writeNode func(*tasks.Node)
	writeNode = func(node *tasks.Node) {
		task := node.Task
		_, _ = fmt.Fprintf(hash, "task\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%t\x00", task.ID, task.Name, task.URL, task.ParentID, task.Status.Name, task.Status.Type, task.Status.OrderIndex, node.Orphan)
		_, _ = io.WriteString(hash, task.Status.Color+"\x00")
		if task.Priority != nil {
			_, _ = fmt.Fprintf(hash, "priority\x00%s\x00%s\x00%s\x00", task.Priority.ID, task.Priority.Name, task.Priority.Color)
		}
		_, _ = io.WriteString(hash, strings.Join(task.Assignees, "\x1f")+"\x00")
		if task.DueDate != nil {
			_, _ = fmt.Fprintf(hash, "due\x00%d\x00", task.DueDate.UnixMilli())
		}
		for _, child := range node.Children {
			writeNode(child)
		}
		_, _ = io.WriteString(hash, "end\x00")
	}
	for _, root := range snapshot.Forest {
		writeNode(root)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func errorsJoin(values ...error) error {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			parts = append(parts, value.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(parts, "; "))
}

func (m *Model) rebuild() {
	forest := tasks.FilterForest(m.snapshot.Forest, m.includeClosed)
	groups := tasks.GroupForest(forest)
	m.lines = nil
	m.nodes = make(map[string]*tasks.Node)
	m.parents = make(map[string]string)
	for _, group := range groups {
		m.lines = append(m.lines, displayLine{kind: groupLine, group: group})
		for _, root := range group.Roots {
			m.appendNode(root, 0, "")
		}
	}
	if m.selectedID == "" || m.nodes[m.selectedID] == nil {
		m.selectedID = m.firstTaskID()
	}
	m.ensureVisible()
}

func (m *Model) appendNode(node *tasks.Node, depth int, parentID string) {
	m.nodes[node.Task.ID] = node
	m.parents[node.Task.ID] = parentID
	m.lines = append(m.lines, displayLine{kind: taskLine, node: node, depth: depth, parentID: parentID})
	if !m.expanded[node.Task.ID] {
		return
	}
	for _, child := range node.Children {
		m.appendNode(child, depth+1, node.Task.ID)
	}
}

func (m *Model) firstTaskID() string {
	for _, line := range m.lines {
		if line.kind == taskLine {
			return line.node.Task.ID
		}
	}
	return ""
}

func (m *Model) selectableIDs() []string {
	ids := make([]string, 0)
	for _, line := range m.lines {
		if line.kind == taskLine {
			ids = append(ids, line.node.Task.ID)
		}
	}
	return ids
}

func (m *Model) move(delta int) {
	ids := m.selectableIDs()
	if len(ids) == 0 {
		return
	}
	current := 0
	for index, id := range ids {
		if id == m.selectedID {
			current = index
			break
		}
	}
	current += delta
	if current < 0 {
		current = 0
	}
	if current >= len(ids) {
		current = len(ids) - 1
	}
	m.selectedID = ids[current]
	m.ensureVisible()
}

func (m *Model) ensureVisible() {
	selectedLine := 0
	for index, line := range m.lines {
		if line.kind == taskLine && line.node.Task.ID == m.selectedID {
			selectedLine = index
			break
		}
	}
	height := m.contentHeight()
	if selectedLine < m.offset {
		m.offset = selectedLine
	}
	if selectedLine >= m.offset+height {
		m.offset = selectedLine - height + 1
	}
	maximum := len(m.lines) - height
	if maximum < 0 {
		maximum = 0
	}
	if m.offset > maximum {
		m.offset = maximum
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *Model) contentHeight() int {
	height := m.height - 5
	if height < 1 {
		return 1
	}
	return height
}

func Run(model *Model) error {
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := program.Run()
	return err
}
