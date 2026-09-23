package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	maskedValue  = "••••••••"
	markOn       = "[x]"
	markOff      = "[ ]"
	cursorMarker = "> "
	noCursor     = "  "
	ellipsis     = "…"
)

var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#B69CFF"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#9A9A9A"}
	colorBorder = lipgloss.AdaptiveColor{Light: "#C4C4C4", Dark: "#4A4A4A"}
	colorError  = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF8A80"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#1B7F3B", Dark: "#7FD99A"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#F2C86B"}
	colorFocus  = lipgloss.AdaptiveColor{Light: "#EDE7FF", Dark: "#2E2650"}
)

type styles struct {
	title        lipgloss.Style
	subtle       lipgloss.Style
	label        lipgloss.Style
	focused      lipgloss.Style
	marked       lipgloss.Style
	masked       lipgloss.Style
	revealed     lipgloss.Style
	pane         lipgloss.Style
	status       lipgloss.Style
	errorText    lipgloss.Style
	warnText     lipgloss.Style
	modal        lipgloss.Style
	choice       lipgloss.Style
	choiceActive lipgloss.Style
	helpKey      lipgloss.Style
	helpDesc     lipgloss.Style
}

func defaultStyles() styles {
	return styles{
		title:        lipgloss.NewStyle().Bold(true).Foreground(colorAccent),
		subtle:       lipgloss.NewStyle().Foreground(colorMuted),
		label:        lipgloss.NewStyle().Bold(true),
		focused:      lipgloss.NewStyle().Bold(true).Background(colorFocus),
		marked:       lipgloss.NewStyle().Foreground(colorOK),
		masked:       lipgloss.NewStyle().Foreground(colorMuted),
		revealed:     lipgloss.NewStyle().Foreground(colorWarn),
		pane:         lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).Padding(0, 1),
		status:       lipgloss.NewStyle().Foreground(colorOK),
		errorText:    lipgloss.NewStyle().Foreground(colorError),
		warnText:     lipgloss.NewStyle().Foreground(colorWarn),
		modal:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(1, 2),
		choice:       lipgloss.NewStyle().Padding(0, 1).Foreground(colorMuted),
		choiceActive: lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(colorAccent).Background(colorFocus),
		helpKey:      lipgloss.NewStyle().Bold(true).Foreground(colorAccent),
		helpDesc:     lipgloss.NewStyle().Foreground(colorMuted),
	}
}

func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= limit {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > limit-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteString(ellipsis)
	return b.String()
}

func padRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func singleLine(s string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`, "\t", `\t`).Replace(s)
}

func scrollWindow(cursor, total, size int) (start, end int) {
	if size <= 0 || total <= 0 {
		return 0, 0
	}
	if total <= size {
		return 0, total
	}
	start = cursor - size/2
	start = max(start, 0)
	start = min(start, total-size)
	return start, start + size
}
