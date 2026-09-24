package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	CursorMarker  = "> "
	NoCursor      = "  "
	Ellipsis      = "…"
	PaneBorder    = 2
	LeftPaneRatio = 0.45
	MinPaneWidth  = 24
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

type Styles struct {
	Title        lipgloss.Style
	Subtle       lipgloss.Style
	Label        lipgloss.Style
	Focused      lipgloss.Style
	Marked       lipgloss.Style
	Masked       lipgloss.Style
	Revealed     lipgloss.Style
	Pane         lipgloss.Style
	PaneActive   lipgloss.Style
	Status       lipgloss.Style
	ErrorText    lipgloss.Style
	WarnText     lipgloss.Style
	Modal        lipgloss.Style
	Choice       lipgloss.Style
	ChoiceActive lipgloss.Style
	HelpKey      lipgloss.Style
	HelpDesc     lipgloss.Style
}

func DefaultStyles() Styles {
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).Padding(0, 1)
	return Styles{
		Title:        lipgloss.NewStyle().Bold(true).Foreground(colorAccent),
		Subtle:       lipgloss.NewStyle().Foreground(colorMuted),
		Label:        lipgloss.NewStyle().Bold(true),
		Focused:      lipgloss.NewStyle().Bold(true).Background(colorFocus),
		Marked:       lipgloss.NewStyle().Foreground(colorOK),
		Masked:       lipgloss.NewStyle().Foreground(colorMuted),
		Revealed:     lipgloss.NewStyle().Foreground(colorWarn),
		Pane:         pane,
		PaneActive:   pane.BorderForeground(colorAccent),
		Status:       lipgloss.NewStyle().Foreground(colorOK),
		ErrorText:    lipgloss.NewStyle().Foreground(colorError),
		WarnText:     lipgloss.NewStyle().Foreground(colorWarn),
		Modal:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(1, 2),
		Choice:       lipgloss.NewStyle().Padding(0, 1).Foreground(colorMuted),
		ChoiceActive: lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(colorAccent).Background(colorFocus),
		HelpKey:      lipgloss.NewStyle().Bold(true).Foreground(colorAccent),
		HelpDesc:     lipgloss.NewStyle().Foreground(colorMuted),
	}
}

func Truncate(s string, limit int) string {
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
	b.WriteString(Ellipsis)
	return b.String()
}

func PadRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func ScrollWindow(cursor, total, size int) (start, end int) {
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
