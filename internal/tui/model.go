package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"botty/internal/browser"
	"botty/internal/tasks"
)

type loadMsg struct {
	snapshot tasks.Snapshot
	err      error
}

type openMsg struct {
	err error
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
}

func NewModel(loader tasks.Loader, opener browser.Opener, includeClosed bool) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		loader:        loader,
		opener:        opener,
		includeClosed: includeClosed,
		expanded:      make(map[string]bool),
		nodes:         make(map[string]*tasks.Node),
		parents:       make(map[string]string),
		width:         100,
		height:        30,
		loading:       true,
		ctx:           ctx,
		cancel:        cancel,
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
		}
		return m, nil
	case openMsg:
		if message.err != nil {
			m.notice = message.err.Error()
		} else {
			m.notice = "Tarea abierta en navegador"
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(message)
	}
	return m, nil
}

func (m *Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "r":
		if !m.loading {
			m.loading = true
			m.err = nil
			m.notice = ""
			return m, m.loadCmd()
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

func (m *Model) sprintSummary() string {
	parts := make([]string, 0, len(m.snapshot.Sprints))
	for _, current := range m.snapshot.Sprints {
		date := current.EffectiveDate().Format("02/01/2006")
		parts = append(parts, fmt.Sprintf("%s (%s)", current.Name, date))
	}
	return strings.Join(parts, "  ·  ")
}

func Run(model *Model) error {
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}
