package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C6AF2"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2B84B"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#3A345E"))
)

func render(m *Model) string {
	width := m.width
	if width < 20 {
		width = 20
	}
	title := titleStyle.Render(fit("clickdown · ClickUp", width))
	mode := "abiertas"
	if m.includeClosed {
		mode = "abiertas + cerradas"
	}
	meta := "modo: " + mode
	if m.loading {
		meta = "Cargando sprints y tareas…"
	}

	noticeText := ""
	noticeStyle := mutedStyle
	if m.err != nil {
		noticeText = m.err.Error()
		noticeStyle = errorStyle
	} else if m.notice != "" {
		noticeText = m.notice
		noticeStyle = warningStyle
	} else if len(m.snapshot.Warnings) > 0 {
		warnings := make([]string, 0, len(m.snapshot.Warnings))
		for _, warning := range m.snapshot.Warnings {
			warnings = append(warnings, warning.Error())
		}
		noticeText = "Carga parcial: " + strings.Join(warnings, "; ")
		noticeStyle = warningStyle
	} else if !m.snapshot.LoadedAt.IsZero() {
		noticeText = "Mostrado " + m.snapshot.LoadedAt.Format("15:04:05")
	}
	notice := noticeStyle.Render(fit(noticeText, width))

	content := make([]string, 0, m.contentHeight())
	if !m.loading && m.err == nil {
		end := m.offset + m.contentHeight()
		if end > len(m.lines) {
			end = len(m.lines)
		}
		for _, line := range m.lines[m.offset:end] {
			content = append(content, renderLine(m, line, width))
		}
		if len(m.lines) == 0 {
			content = append(content, mutedStyle.Render("Sin tareas para modo seleccionado"))
		}
	}
	for len(content) < m.contentHeight() {
		content = append(content, "")
	}

	footer := mutedStyle.Render(fit("↑/↓ navegar  espacio expandir  a cerradas  Enter abrir  r recargar  q salir", width))
	return strings.Join([]string{
		title,
		fit(meta, width),
		notice,
		strings.Join(content, "\n"),
		fit(footer, width),
	}, "\n")
}

func renderLine(m *Model, line displayLine, width int) string {
	if line.kind == groupLine {
		color := lipgloss.Color(line.group.Status.Color)
		if line.group.Status.Color == "" {
			color = lipgloss.Color("#7C6AF2")
		}
		label := fmt.Sprintf("%s (%d)", strings.ToUpper(line.group.Status.Name), len(line.group.Roots))
		return lipgloss.NewStyle().Bold(true).Foreground(color).Render(fit(label, width))
	}

	task := line.node.Task
	marker := "·"
	if len(line.node.Children) > 0 {
		marker = "▸"
		if m.expanded[task.ID] {
			marker = "▾"
		}
	}
	prefix := "  "
	if task.ID == m.selectedID {
		prefix = "› "
	}
	left := prefix + strings.Repeat("  ", line.depth) + marker + " " + task.Name
	badges := make([]string, 0, 5)
	if line.depth > 0 {
		badges = append(badges, task.Status.Name)
	}
	if task.Priority != nil && task.Priority.Name != "" {
		color := priorityColor(task.Priority.Name, task.Priority.Color)
		badges = append(badges, lipgloss.NewStyle().Bold(true).Foreground(color).Render(task.Priority.Name))
	}
	if task.ID == m.selectedID && len(task.Assignees) > 0 {
		assignees := task.Assignees
		if len(assignees) > 2 {
			assignees = append(append([]string{}, assignees[:2]...), fmt.Sprintf("+%d", len(task.Assignees)-2))
		}
		badges = append(badges, strings.Join(assignees, ", "))
	}
	if task.DueDate != nil {
		badges = append(badges, task.DueDate.Format("02/01"))
	}
	if line.node.Orphan {
		badges = append(badges, "padre no disponible")
	} else if line.node.Promoted {
		badges = append(badges, "padre cerrado")
	}
	right := ""
	if len(badges) != 0 {
		right = "[" + strings.Join(badges, " · ") + "]"
		maximumRight := width / 2
		if lipgloss.Width(right) > maximumRight {
			inner := strings.TrimSuffix(strings.TrimPrefix(right, "["), "]")
			right = "[" + fit(inner, maximumRight-2) + "]"
		}
	}
	gap := 0
	if right != "" {
		left = fit(left, width-lipgloss.Width(right)-1)
		gap = width - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
	} else {
		left = fit(left, width)
	}
	text := left + strings.Repeat(" ", gap) + right
	if task.ID == m.selectedID {
		return selectedStyle.Width(width).Render(text)
	}
	return text
}

func priorityColor(name, apiColor string) lipgloss.Color {
	if strings.TrimSpace(apiColor) != "" {
		return lipgloss.Color(apiColor)
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "urgent", "urgente":
		return lipgloss.Color("#F50000")
	case "high", "alta":
		return lipgloss.Color("#FF8C00")
	case "normal":
		return lipgloss.Color("#F8AE00")
	case "low", "baja":
		return lipgloss.Color("#6BC950")
	default:
		return lipgloss.Color("#7C6AF2")
	}
}

func fit(value string, width int) string {
	if width <= 1 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}
