package theme

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

const helpKeyColumn = 12

func FullHelp(st Styles, keys KeyMap, width, height int) string {
	groups := keys.AllGroups()
	split := (len(groups) + 1) / 2
	left := helpColumn(st, groups[:split])
	right := helpColumn(st, groups[split:])
	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)
	content := st.Title.Render("Atalhos") + "\n\n" + columns
	return st.Pane.Width(max(width-PaneBorder, 1)).Height(max(height-PaneBorder, 1)).Render(content)
}

func helpColumn(st Styles, groups [][]key.Binding) string {
	var lines []string
	for i, group := range groups {
		if i > 0 {
			lines = append(lines, "")
		}
		for _, b := range group {
			h := b.Help()
			lines = append(lines, st.HelpKey.Render(PadRight(h.Key, helpKeyColumn))+st.HelpDesc.Render(h.Desc))
		}
	}
	return strings.Join(lines, "\n")
}
