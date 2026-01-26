package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/valentindosimont/ccmanager/internal/claude"
	"github.com/valentindosimont/ccmanager/internal/game"
)

// Colors
var (
	colorPrimary   = lipgloss.Color("#00BFFF")
	colorSecondary = lipgloss.Color("#FFD700")
	colorUrgent    = lipgloss.Color("#FF4444")
	colorSuccess   = lipgloss.Color("#44FF44")
	colorMuted     = lipgloss.Color("#666666")
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary)

	statStyle = lipgloss.NewStyle().
			Foreground(colorPrimary)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSuccess)

	urgentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorUrgent)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)
)

// View renders the UI
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.confirmQuit {
		return "\n  Close all tmux sessions?\n\n  [y] Yes, kill sessions  [n] No, keep running  [c] Cancel\n"
	}

	if m.pathPickerMode {
		return m.viewPathPicker()
	}

	if m.inputMode || m.renameMode {
		return m.viewInputOverlay()
	}

	if m.showHelp {
		return m.viewHelp()
	}

	if m.showStats {
		return m.viewStats()
	}

	if m.showActivity {
		return m.viewActivityOverlay()
	}

	// Calculate layout dimensions
	innerWidth := m.width - 2 // account for outer border

	// Fixed heights
	const (
		headerHeight   = 1
		groupsHeight   = 1
		helpBarHeight  = 1
		minActivityLog = 4
		borderOverhead = 4 // horizontal dividers
	)

	// Calculate prompt panel height (dynamic based on textarea content)
	promptLines := 1 // header
	if m.promptMode {
		promptLines += strings.Count(m.promptField.Value(), "\n") + 1
	} else {
		promptLines += 1 // hint line
	}
	activityHeight := max(minActivityLog, (m.height-10)/5)
	promptHeight := promptLines + max(0, activityHeight-promptLines)
	if promptHeight < promptLines+2 {
		promptHeight = promptLines + 2
	}
	maxPromptHeight := m.height / 2
	if promptHeight > maxPromptHeight {
		promptHeight = maxPromptHeight
	}
	// Frame interior is m.height - 2; content must fit within it
	// content = header(1) + groups(1) + helpBar(1) + 4 dividers + mainHeight + promptHeight
	mainHeight := m.height - headerHeight - groupsHeight - promptHeight - helpBarHeight - borderOverhead - 4
	if mainHeight < 5 {
		mainHeight = 5
	}

	// Build content sections
	header := m.viewHeader(innerWidth)
	groups := m.viewControlGroups(innerWidth)
	mainContent := m.viewMainContent(innerWidth, mainHeight)
	prompt := m.viewPromptPanel(innerWidth, promptHeight)
	helpBar := m.viewHelpBar(innerWidth)

	// Horizontal divider
	divider := lipgloss.NewStyle().Foreground(colorPrimary).Render(strings.Repeat("─", innerWidth))

	// Join all sections
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		divider,
		groups,
		divider,
		mainContent,
		divider,
		prompt,
		divider,
		helpBar,
	)

	// Wrap in outer frame
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Width(innerWidth).
		Height(m.height - 2)

	view := frame.Render(content)

	// Overlay notification popup if active
	if m.showNotification {
		popup := m.viewNotification()
		view = m.overlayPopup(view, popup)
	}

	return view
}

func (m *Model) viewHeader(width int) string {
	title := titleStyle.Render("CCMANAGER")

	apm := statStyle.Render(fmt.Sprintf("APM: %d", m.apm))

	var usageStr string
	if m.selected >= 0 && m.selected < len(m.sessions) {
		if sess := m.sessions[m.selected]; sess != nil && sess.Usage != nil {
			usageStr = formatUsageCompact(sess.Usage.TotalUsage.TotalInput(),
				sess.Usage.TotalUsage.OutputTokens, sess.Usage.EstimatedCost)
		}
	}

	streakStr := fmt.Sprintf("STREAK: x%.1f", m.streakMult)
	streak := statStyle.Render(streakStr)
	if m.streakMult >= 5.0 {
		streak = lipgloss.NewStyle().Bold(true).Foreground(colorSecondary).Render(streakStr)
	}

	score := statStyle.Render(fmt.Sprintf("SCORE: %s", formatScore(m.score)))

	pomodoroStr := m.formatPomodoro()
	pomodoro := statStyle.Render(pomodoroStr)
	if m.pomodoroState == game.PomodoroWork {
		pomodoro = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess).Render(pomodoroStr)
	}

	// Build stats - usage first (if available), then others
	var statParts []string
	if usageStr != "" {
		statParts = append(statParts, statStyle.Render(usageStr))
	}
	statParts = append(statParts, apm, streak, score, pomodoro)
	stats := strings.Join(statParts, "  │  ")
	statsWidth := lipgloss.Width(stats)
	titleWidth := lipgloss.Width(title)

	padding := width - titleWidth - statsWidth - 2
	if padding < 1 {
		padding = 1
	}

	return " " + title + strings.Repeat(" ", padding) + stats + " "
}

func (m *Model) formatPomodoro() string {
	if m.pomodoroState == game.PomodoroStopped {
		return "🍅 --:--"
	}

	mins := int(m.pomodoroRemain.Minutes())
	secs := int(m.pomodoroRemain.Seconds()) % 60

	if m.pomodoroState == game.PomodoroPaused {
		return fmt.Sprintf("⏸ %02d:%02d", mins, secs)
	}

	stateStr := ""
	switch m.pomodoroState {
	case game.PomodoroWork:
		stateStr = "🍅"
	case game.PomodoroShortBreak:
		stateStr = "☕"
	case game.PomodoroLongBreak:
		stateStr = "🌴"
	}
	return fmt.Sprintf("%s %02d:%02d", stateStr, mins, secs)
}

func (m *Model) viewControlGroups(width int) string {
	var parts []string
	parts = append(parts, "GROUPS:")

	for i := 1; i <= 9; i++ {
		session := m.engine.ControlGroups().Get(i)
		if session == "" {
			parts = append(parts, mutedStyle.Render(fmt.Sprintf("[%d]", i)))
		} else {
			if m.selected < len(m.sessions) && m.engine.ControlGroups().Contains(i, m.sessions[m.selected].Name) {
				parts = append(parts, selectedStyle.Render(fmt.Sprintf("[%d]●", i)))
			} else {
				parts = append(parts, statStyle.Render(fmt.Sprintf("[%d]●", i)))
			}
		}
	}

	// Zero group
	session := m.engine.ControlGroups().Get(10)
	if session == "" {
		parts = append(parts, mutedStyle.Render("[0]"))
	} else {
		parts = append(parts, statStyle.Render("[0]●"))
	}

	left := " " + strings.Join(parts, " ")

	// Focus indicator on the right
	focusStr := ""
	if m.focused != "" {
		focusStr = fmt.Sprintf("FOCUS: %s ", m.focused)
	}

	padding := width - lipgloss.Width(left) - lipgloss.Width(focusStr)
	if padding < 1 {
		padding = 1
	}

	return left + strings.Repeat(" ", padding) + focusStr
}

func (m *Model) viewMainContent(width, height int) string {
	if m.sessionListHidden {
		return m.viewPreview(width, height)
	}

	if width < 60 {
		return m.viewSessionList(width-2, height)
	}

	// Calculate column widths
	widthPct := m.config.UI.SessionListWidthPct
	if widthPct < 20 {
		widthPct = 20
	}
	if widthPct > 50 {
		widthPct = 50
	}
	sessionListWidth := width * widthPct / 100
	if sessionListWidth < 25 {
		sessionListWidth = 25
	}
	previewWidth := width - sessionListWidth - 3 // 3 for divider with padding

	sessionList := m.viewSessionList(sessionListWidth, height)
	preview := m.viewPreview(previewWidth, height)

	// Vertical divider
	dividerLines := make([]string, height)
	for i := range dividerLines {
		dividerLines[i] = lipgloss.NewStyle().Foreground(colorPrimary).Render("│")
	}
	divider := strings.Join(dividerLines, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, sessionList, divider, preview)
}

func (m *Model) viewSessionList(width, height int) string {
	var lines []string

	// Header
	header := sectionHeaderStyle.Render(" SESSIONS")
	lines = append(lines, header)
	lines = append(lines, " "+strings.Repeat("─", width-2))

	if len(m.sessions) == 0 {
		lines = append(lines, mutedStyle.Render("  No Claude sessions found"))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:height], "\n")
	}

	// Calculate visible window
	visibleCount := height - 2 // minus header and divider
	if visibleCount < 1 {
		visibleCount = 1
	}

	startIdx := 0
	if m.selected >= visibleCount {
		startIdx = m.selected - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(m.sessions) {
		endIdx = len(m.sessions)
		startIdx = max(0, endIdx-visibleCount)
	}

	// Render visible sessions
	for i := startIdx; i < endIdx; i++ {
		sess := m.sessions[i]

		cursor := "  "
		if i == m.selected {
			cursor = "▸ "
		}

		groups := m.engine.ControlGroups().GroupsForSession(sess.Name)
		groupStr := "   "
		if len(groups) > 0 {
			groupStr = fmt.Sprintf("[%d]", groups[0])
		}

		stateIcon := m.stateIcon(sess.State)
		stateStr := sess.State.String()

		elapsed := time.Since(sess.Created)
		elapsedStr := formatDuration(elapsed)

		// Format cost if available
		costStr := ""
		if sess.Usage != nil && sess.Usage.EstimatedCost > 0 {
			costStr = fmt.Sprintf("$%.2f", sess.Usage.EstimatedCost)
		}

		// Calculate available width for session name
		nameWidth := width - 32 // cursor(2) + group(4) + icon(2) + state(8) + elapsed(6) + cost(7) + padding
		if nameWidth < 8 {
			nameWidth = 8
		}

		displayName := sess.Name
		if repoName, ok := m.workspaceRepos[sess.Name]; ok {
			displayName = fmt.Sprintf("%s (%s)", sess.Name, repoName)
		}

		line := fmt.Sprintf("%s%-*s %s %s%-8s %5s %6s",
			cursor,
			nameWidth,
			truncate(displayName, nameWidth),
			groupStr,
			stateIcon,
			stateStr,
			elapsedStr,
			costStr,
		)

		if i == m.selected {
			line = selectedStyle.Render(line)
		} else if sess.State == claude.StateUrgent {
			line = urgentStyle.Render(line)
		} else if sess.State == claude.StateIdle {
			line = mutedStyle.Render(line)
		}

		lines = append(lines, line)
	}

	// Add scroll indicator if needed
	if len(m.sessions) > visibleCount {
		scrollInfo := mutedStyle.Render(fmt.Sprintf(" [%d-%d of %d]", startIdx+1, endIdx, len(m.sessions)))
		for len(lines) < height-1 {
			lines = append(lines, "")
		}
		lines = append(lines, scrollInfo)
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

func (m *Model) stateIcon(state claude.SessionState) string {
	switch state {
	case claude.StateActive:
		return "●"
	case claude.StateIdle:
		return "○"
	case claude.StateThinking:
		return "◐"
	case claude.StateUrgent:
		return "⚡"
	default:
		return "?"
	}
}

func (m *Model) viewPreview(width, height int) string {
	var lines []string

	// Header
	header := sectionHeaderStyle.Render(" PREVIEW")
	lines = append(lines, header)
	lines = append(lines, " "+strings.Repeat("─", width-2))

	if m.selected >= len(m.sessions) {
		lines = append(lines, mutedStyle.Render(" No session selected"))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:height], "\n")
	}

	sess := m.sessions[m.selected]

	// Status line
	statusLine := fmt.Sprintf(" %s %s", m.stateIcon(sess.State), sess.State)
	if sess.State == claude.StateThinking && sess.ThinkingTime > 0 {
		statusLine += fmt.Sprintf(" (%s)", formatDuration(sess.ThinkingTime))
	}
	if sess.Tokens > 0 {
		statusLine += fmt.Sprintf(" · ↓ %s tokens", formatTokens(sess.Tokens))
	}

	// Add usage info if available
	if sess.Usage != nil {
		usageStr := formatUsageCompact(sess.Usage.TotalUsage.TotalInput(), sess.Usage.TotalUsage.OutputTokens, sess.Usage.EstimatedCost)
		statusLine += " · " + usageStr
	}
	lines = append(lines, statStyle.Render(statusLine))

	// Get content: use direct capture for selected (with cache fallback), cache for others
	var content string
	if m.selectedPreviewContent != "" {
		content = m.selectedPreviewContent
	} else {
		content = m.previewCache[sess.Name]
	}
	if content != "" {
		contentLines := strings.Split(strings.TrimSpace(content), "\n")
		availableHeight := height - len(lines) - 1
		scrollPos := m.previewScrollPos[sess.Name]

		// Calculate visible window (scrollPos is offset from bottom)
		totalLines := len(contentLines)
		end := totalLines - scrollPos
		if end < 0 {
			end = 0
		}
		start := end - availableHeight
		if start < 0 {
			start = 0
		}

		// Clamp scrollPos if content shrunk
		if scrollPos > 0 && end <= start {
			m.previewScrollPos[sess.Name] = max(0, totalLines-availableHeight)
			end = totalLines - m.previewScrollPos[sess.Name]
			start = max(0, end-availableHeight)
		}

		for _, line := range contentLines[start:end] {
			if ansi.StringWidth(line) > width-2 {
				line = ansi.Truncate(line, width-2, "")
			}
			lines = append(lines, " "+line)
			if len(lines) >= height-1 {
				break
			}
		}

		// Add scroll indicator if not at bottom
		if scrollPos > 0 && totalLines > availableHeight {
			pct := 100 - (scrollPos * 100 / (totalLines - availableHeight + 1))
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			indicator := mutedStyle.Render(fmt.Sprintf(" [%d%%]", pct))
			for len(lines) < height-1 {
				lines = append(lines, "")
			}
			lines = append(lines, indicator)
		}
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

func (m *Model) viewPromptPanel(width, height int) string {
	var lines []string

	// Header with target session
	sessionName := ""
	if m.selected < len(m.sessions) {
		sessionName = m.sessions[m.selected].Name
	}

	if m.interactiveMode && sessionName != "" {
		header := sectionHeaderStyle.Render(fmt.Sprintf(" INTERACTIVE → %s", sessionName))
		lines = append(lines, header)
	} else if sessionName != "" {
		header := sectionHeaderStyle.Render(fmt.Sprintf(" PROMPT → %s", sessionName))
		lines = append(lines, header)
	} else {
		header := sectionHeaderStyle.Render(" PROMPT")
		lines = append(lines, header)
	}

	// Show input or hint
	if m.promptMode {
		promptView := m.promptField.View()
		promptLines := strings.Split(promptView, "\n")
		for _, pl := range promptLines {
			lines = append(lines, " > "+pl)
		}
	} else {
		hint := mutedStyle.Render(" Press [i] or [/] to type a command")
		lines = append(lines, hint)
	}

	// Show last few activity entries in muted style below
	activityStart := len(lines)
	maxActivityEntries := height - activityStart
	if maxActivityEntries < 0 {
		maxActivityEntries = 0
	}

	for i := 0; i < maxActivityEntries && i < len(m.activityLog); i++ {
		entry := m.activityLog[i]
		timeStr := entry.Time.Format("15:04")
		msg := entry.Message
		maxMsgLen := width - 10
		if maxMsgLen > 0 && len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen-1] + "…"
		}
		line := fmt.Sprintf(" %s %s", timeStr, msg)
		lines = append(lines, mutedStyle.Render(line))
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:height], "\n")
}

func (m *Model) viewActivityOverlay() string {
	var lines []string

	title := titleStyle.Render("ACTIVITY LOG")
	lines = append(lines, title)
	lines = append(lines, "")

	// Show up to 15 recent entries
	maxEntries := 15
	if len(m.activityLog) < maxEntries {
		maxEntries = len(m.activityLog)
	}

	for i := 0; i < maxEntries; i++ {
		entry := m.activityLog[i]
		timeStr := entry.Time.Format("15:04:05")
		sessionStr := ""
		if entry.Session != "" {
			sessionStr = fmt.Sprintf("[%s] ", entry.Session)
		}
		line := fmt.Sprintf("%s %s%s", timeStr, sessionStr, entry.Message)
		if len(line) > 60 {
			line = line[:59] + "…"
		}
		lines = append(lines, line)
	}

	if len(m.activityLog) == 0 {
		lines = append(lines, mutedStyle.Render("No activity yet"))
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Press any key to close"))

	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Render(content)
}

func (m *Model) viewHelpBar(width int) string {
	var help string
	if m.interactiveMode {
		help = "[↑↓/jk] select  [Enter] confirm  [y/n] quick  [i] text  [Esc] exit"
	} else {
		help = "[↑↓] nav  [i] prompt  [x] cancel  [c] int  [dd] del  [n] new  [e] editor  [?] help  [q] quit"
	}
	padding := (width - len(help)) / 2
	if padding < 1 {
		padding = 1
	}
	return helpStyle.Render(strings.Repeat(" ", padding) + help)
}

func (m *Model) viewHelp() string {
	help := `
              CCMANAGER HELP
──────────────────────────────────────────
NAVIGATION
  ↑/k, ↓/j    Move selection
  Enter       Focus session (switch tmux)
  Tab         Cycle sessions

SESSIONS
  n           Create new session
  r           Rename selected session
  dd          Delete selected session
  e           Open editor in session dir

PREVIEW
  Ctrl+U      Scroll up
  Ctrl+D      Scroll down
  G           Jump to bottom
  [           Toggle session list

PROMPT
  i, /        Enter prompt mode
  Enter       Send command to session
  Esc         Exit prompt mode
  Shift+Tab   Cycle Claude mode
  x           Cancel Claude task (Escape)
  c           Interrupt Claude (Ctrl+C)
  D           Show activity overlay

CONTROL GROUPS
  1-9, 0      Tap: cycle, Double-tap: focus
  g + 1-9     Assign selected to group

GAME
  p           Start/pause pomodoro
  P           Stop pomodoro
  s           Show statistics

GENERAL
  ?           Toggle help
  q           Quit

         Press any key to close
`
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Render(help)
}

func (m *Model) viewStats() string {
	stats := fmt.Sprintf(`
            SESSION STATISTICS
──────────────────────────────────────────
Today
  Score:          %s
  APM (current):  %d
  Best Streak:    x%.1f

         Press any key to close
`, formatScore(m.score), m.apm, m.streakMult)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Render(stats)
}

func (m *Model) viewInputOverlay() string {
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(50)

	var title, help string
	if m.renameMode {
		title = titleStyle.Render("Rename Session:")
		help = helpStyle.Render("[Enter] Rename  [Esc] Cancel")
	} else {
		if m.workspaceMode && m.selectedPath != "" {
			repoName := filepath.Base(m.selectedPath)
			title = titleStyle.Render(fmt.Sprintf("New Session for %s:", repoName))
		} else {
			title = titleStyle.Render("New Session Name:")
		}
		help = helpStyle.Render("[Enter] Create  [Esc] Cancel")
	}
	input := m.inputField.View()

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		input,
		"",
		help,
	)

	return inputBox.Render(content)
}

func (m *Model) viewPathPicker() string {
	var wsStatus string
	if m.workspaceMode {
		wsStatus = "[w] workspace: ON"
	} else {
		wsStatus = "[w] workspace: off"
	}
	help := helpStyle.Render(wsStatus + "  [Enter] select  [Esc] cancel")

	listView := m.pathPickerList.View()
	content := lipgloss.JoinVertical(lipgloss.Left, listView, "", help)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(m.width - 4).
		Height(m.height - 4).
		Render(content)
}

func (m *Model) viewNotification() string {
	notification := `
        POMODORO COMPLETE!

        Time to take a break.
        Drink some water!

        Press any key to dismiss
`
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSuccess).
		Padding(1, 2).
		Render(notification)
}

func (m *Model) overlayPopup(background, popup string) string {
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{}),
	)
}

// Helper functions

func formatScore(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

func formatUsageCompact(inputTokens, outputTokens int64, cost float64) string {
	inStr := formatTokensLarge(inputTokens)
	outStr := formatTokensLarge(outputTokens)
	return fmt.Sprintf("↓%s ↑%s $%.2f", inStr, outStr, cost)
}

func formatTokensLarge(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
}
