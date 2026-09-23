package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

const (
	detailHeaderLines = 5
	maxKeyColumn      = 40
)

type detailModel struct {
	env      vault.Env
	cursor   int
	revealed bool
}

func newDetailModel(env vault.Env) detailModel {
	return detailModel{env: env}
}

func (d detailModel) move(delta int) detailModel {
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

func (d detailModel) toggleReveal() detailModel {
	if len(d.env.Vars) == 0 {
		return d
	}
	d.revealed = !d.revealed
	return d
}

func (d detailModel) focusedVar() (vault.Var, bool) {
	if d.cursor < 0 || d.cursor >= len(d.env.Vars) {
		return vault.Var{}, false
	}
	return d.env.Vars[d.cursor], true
}

func copyValueCmd(write func(string) error, v vault.Var) tea.Cmd {
	return func() tea.Msg {
		if err := write(v.Value); err != nil {
			return statusMsg{err: fmt.Errorf("copiar %s: %w", v.Key, err)}
		}
		return statusMsg{text: fmt.Sprintf("valor de %s copiado para o clipboard", v.Key)}
	}
}

func (d detailModel) view(st styles, width, height int) string {
	inner := max(width-4, 10)
	lines := []string{
		st.title.Render(truncate(d.env.Name, inner)),
		st.label.Render("descrição: ") + truncate(orDash(d.env.Description), max(inner-11, 1)),
		st.label.Render("tags: ") + truncate(orDash(strings.Join(d.env.Tags, ", ")), max(inner-6, 1)),
		"",
	}
	if len(d.env.Vars) == 0 {
		lines = append(lines, st.subtle.Render("env sem chaves"))
		return st.pane.Width(max(width-paneBorder, 1)).Height(max(height-paneBorder, 1)).Render(strings.Join(lines, "\n"))
	}
	keyWidth := len("CHAVE")
	for _, v := range d.env.Vars {
		keyWidth = max(keyWidth, lipgloss.Width(v.Key))
	}
	keyWidth = min(keyWidth, maxKeyColumn, max(inner/2, 5))
	valueWidth := max(inner-keyWidth-lipgloss.Width(cursorMarker)-2, 4)
	lines = append(lines, st.subtle.Render(noCursor+padRight("CHAVE", keyWidth)+"  VALOR"))
	rows := max(height-paneBorder-detailHeaderLines, 1)
	start, end := scrollWindow(d.cursor, len(d.env.Vars), rows)
	for i := start; i < end; i++ {
		lines = append(lines, d.row(st, i, keyWidth, valueWidth))
	}
	return st.pane.Width(max(width-paneBorder, 1)).Height(max(height-paneBorder, 1)).Render(strings.Join(lines, "\n"))
}

func (d detailModel) row(st styles, i, keyWidth, valueWidth int) string {
	v := d.env.Vars[i]
	focused := i == d.cursor
	prefix := noCursor
	if focused {
		prefix = cursorMarker
	}
	value := st.masked.Render(maskedValue)
	if focused && d.revealed {
		value = st.revealed.Render(truncate(singleLine(v.Value), valueWidth))
	}
	line := prefix + padRight(truncate(v.Key, keyWidth), keyWidth) + "  " + value
	if focused {
		return st.focused.Render(line)
	}
	return line
}
