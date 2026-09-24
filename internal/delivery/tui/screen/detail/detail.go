package detail

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	headerLines  = 5
	maxKeyColumn = 40
	keyHeading   = "CHAVE"
)

type Intent int

const (
	IntentNone Intent = iota
	IntentBack
	IntentHelp
)

type CopiedMsg struct {
	Status string
	Err    error
}

type Model struct {
	keys     theme.KeyMap
	write    func(string) error
	env      vaultusecase.EnvView
	cursor   int
	revealed bool
}

func New(env vaultusecase.EnvView, write func(string) error) Model {
	return Model{keys: theme.DefaultKeyMap(), write: write, env: env}
}

func (d Model) Env() vaultusecase.EnvView {
	return d.env
}

func (d Model) Refresh(envs []vaultusecase.EnvView) (Model, bool) {
	for _, env := range envs {
		if env.Name == d.env.Name {
			cursor := min(d.cursor, max(len(env.Vars)-1, 0))
			refreshed := New(env, d.write)
			refreshed.cursor = cursor
			return refreshed, true
		}
	}
	return Model{}, false
}

func (d Model) Update(msg tea.KeyMsg) (Model, tea.Cmd, Intent) {
	switch {
	case key.Matches(msg, d.keys.Quit, d.keys.Back):
		return d, nil, IntentBack
	case key.Matches(msg, d.keys.Up):
		d = d.move(-1)
	case key.Matches(msg, d.keys.Down):
		d = d.move(1)
	case key.Matches(msg, d.keys.Reveal):
		d = d.toggleReveal()
	case key.Matches(msg, d.keys.Help):
		return d, nil, IntentHelp
	case key.Matches(msg, d.keys.Copy):
		if v, ok := d.focusedVar(); ok {
			return d, copyValueCmd(d.write, v), IntentNone
		}
	}
	return d, nil, IntentNone
}

func (d Model) move(delta int) Model {
	if len(d.env.Vars) == 0 {
		return d
	}
	next := min(max(d.cursor+delta, 0), len(d.env.Vars)-1)
	if next != d.cursor {
		d.revealed = false
	}
	d.cursor = next
	return d
}

func (d Model) toggleReveal() Model {
	if len(d.env.Vars) == 0 {
		return d
	}
	d.revealed = !d.revealed
	return d
}

func (d Model) focusedVar() (vaultusecase.VarView, bool) {
	if d.cursor < 0 || d.cursor >= len(d.env.Vars) {
		return vaultusecase.VarView{}, false
	}
	return d.env.Vars[d.cursor], true
}

func copyValueCmd(write func(string) error, v vaultusecase.VarView) tea.Cmd {
	return func() tea.Msg {
		if err := write(v.Value); err != nil {
			return CopiedMsg{Err: fmt.Errorf("copiar %s: %w", v.Key, err)}
		}
		return CopiedMsg{Status: fmt.Sprintf("valor de %s copiado para o clipboard", v.Key)}
	}
}

func (d Model) View(st theme.Styles, width, height int) string {
	inner := max(width-4, 10)
	header := viewmodel.Header(d.env)
	lines := []string{
		st.Title.Render(theme.Truncate(header.Name, inner)),
		st.Label.Render("descrição: ") + theme.Truncate(header.Description, max(inner-11, 1)),
		st.Label.Render("tags: ") + theme.Truncate(header.Tags, max(inner-6, 1)),
		"",
	}
	pane := st.Pane.Width(max(width-theme.PaneBorder, 1)).Height(max(height-theme.PaneBorder, 1))
	rows := viewmodel.DetailRows(d.env, d.cursor, d.revealed)
	if len(rows) == 0 {
		lines = append(lines, st.Subtle.Render("env sem chaves"))
		return pane.Render(strings.Join(lines, "\n"))
	}
	keyWidth := len(keyHeading)
	for _, row := range rows {
		keyWidth = max(keyWidth, lipgloss.Width(row.Key))
	}
	keyWidth = min(keyWidth, maxKeyColumn, max(inner/2, 5))
	valueWidth := max(inner-keyWidth-lipgloss.Width(theme.CursorMarker)-2, 4)
	lines = append(lines, st.Subtle.Render(theme.NoCursor+theme.PadRight(keyHeading, keyWidth)+"  VALOR"))
	visible := max(height-theme.PaneBorder-headerLines, 1)
	start, end := theme.ScrollWindow(d.cursor, len(rows), visible)
	for i := start; i < end; i++ {
		lines = append(lines, renderRow(st, rows[i], i == d.cursor, keyWidth, valueWidth))
	}
	return pane.Render(strings.Join(lines, "\n"))
}

func renderRow(st theme.Styles, row viewmodel.VarRow, focused bool, keyWidth, valueWidth int) string {
	prefix := theme.NoCursor
	if focused {
		prefix = theme.CursorMarker
	}
	value := st.Masked.Render(row.Value)
	if row.Revealed {
		value = st.Revealed.Render(theme.Truncate(row.Value, valueWidth))
	}
	line := prefix + theme.PadRight(theme.Truncate(row.Key, keyWidth), keyWidth) + "  " + value
	if focused {
		return st.Focused.Render(line)
	}
	return line
}
