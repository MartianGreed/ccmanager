package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type pathItem struct {
	path   string
	source string // "config", "recent", "cwd"
}

func (i pathItem) Title() string       { return filepath.Base(i.path) }
func (i pathItem) Description() string { return i.path }
func (i pathItem) FilterValue() string { return i.path }

type pathDelegate struct {
	styles pathDelegateStyles
}

type pathDelegateStyles struct {
	normal   lipgloss.Style
	selected lipgloss.Style
	dimmed   lipgloss.Style
}

func newPathDelegate() pathDelegate {
	return pathDelegate{
		styles: pathDelegateStyles{
			normal:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
			selected: lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Bold(true),
			dimmed:   lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")),
		},
	}
}

func (d pathDelegate) Height() int                             { return 2 }
func (d pathDelegate) Spacing() int                            { return 0 }
func (d pathDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d pathDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(pathItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	width := m.Width() - 4

	var title, desc string
	if isSelected {
		title = d.styles.selected.Render("▸ " + i.Title())
		desc = d.styles.selected.Render("  " + truncatePath(i.path, width))
	} else {
		title = d.styles.normal.Render("  " + i.Title())
		desc = d.styles.dimmed.Render("  " + truncatePath(i.path, width))
	}

	sourceTag := ""
	switch i.source {
	case "config":
		sourceTag = d.styles.dimmed.Render(" [cfg]")
	case "recent":
		sourceTag = d.styles.dimmed.Render(" [recent]")
	case "cwd":
		sourceTag = d.styles.dimmed.Render(" [cwd]")
	}

	_, _ = fmt.Fprintf(w, "%s%s\n%s\n", title, sourceTag, desc)
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	if maxLen <= 3 {
		return "..."
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= 2 {
		return path[:maxLen-3] + "..."
	}
	return "..." + path[len(path)-maxLen+3:]
}
