package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C6AF2"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2B84B"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
)

func render(m *Model) string {
	width := m.width
	if width < 20 {
		width = 20
	}
	title := titleStyle.Render(fit("botty · ClickUp", width))
	mode := "abiertas"
	if m.includeClosed {
		mode = "abiertas + cerradas"
	}
	meta := fmt.Sprintf("%s  ·  modo: %s", m.sprintSummary(), mode)
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
		noticeText = "Actualizado " + m.snapshot.LoadedAt.Format("15:04:05")
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
	text := prefix + strings.Repeat("  ", line.depth) + marker + " " + task.Name
	badges := make([]string, 0, 5)
	if len(task.SprintNames) > 0 {
		badges = append(badges, strings.Join(task.SprintNames, "/"))
	}
	if line.depth > 0 {
		badges = append(badges, task.Status.Name)
	}
	if task.Priority != nil && task.Priority.Name != "" {
		badges = append(badges, task.Priority.Name)
	}
	if len(task.Assignees) > 0 {
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
	if len(badges) > 0 {
		text += "  [" + strings.Join(badges, " · ") + "]"
	}
	text = fit(text, width)
	if task.ID == m.selectedID {
		return selectedStyle.Width(width).Render(text)
	}
	return text
}

func fit(value string, width int) string {
	if width <= 1 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
