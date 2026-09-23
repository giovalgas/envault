package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

const (
	timeLayout    = "2006-01-02 15:04"
	leftPaneRatio = 0.45
	minPaneWidth  = 24
	paneBorder    = 2
)

type listModel struct {
	envs      []vault.Env
	visible   []int
	cursor    int
	marked    []string
	filter    textinput.Model
	filtering bool
	loaded    bool
	failed    bool
}

func newListModel() listModel {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "nome, descrição, tag ou chave"
	ti.CharLimit = 64
	ti.Cursor.SetMode(cursor.CursorStatic)
	return listModel{filter: ti}
}

func (l listModel) setEnvs(envs []vault.Env, focus string) listModel {
	if focus == "" {
		if current, ok := l.focused(); ok {
			focus = current.Name
		}
	}
	l.envs = envs
	l.loaded = true
	l.failed = false
	l.marked = slices.DeleteFunc(slices.Clone(l.marked), func(name string) bool {
		return !l.hasEnv(name)
	})
	l = l.applyFilter()
	l = l.focusName(focus)
	return l
}

func (l listModel) markFailed() listModel {
	l.failed = true
	return l
}

func (l listModel) focusName(name string) listModel {
	for i, idx := range l.visible {
		if l.envs[idx].Name == name {
			l.cursor = i
			return l
		}
	}
	l.cursor = min(l.cursor, max(len(l.visible)-1, 0))
	return l
}

func (l listModel) applyFilter() listModel {
	query := strings.ToLower(strings.TrimSpace(l.filter.Value()))
	l.visible = l.visible[:0:0]
	for i, env := range l.envs {
		if query == "" || matchesFilter(env, query) {
			l.visible = append(l.visible, i)
		}
	}
	l.cursor = min(l.cursor, max(len(l.visible)-1, 0))
	return l
}

func matchesFilter(env vault.Env, query string) bool {
	if strings.Contains(strings.ToLower(env.Name), query) ||
		strings.Contains(strings.ToLower(env.Description), query) {
		return true
	}
	for _, tag := range env.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	for _, v := range env.Vars {
		if strings.Contains(strings.ToLower(v.Key), query) {
			return true
		}
	}
	return false
}

func (l listModel) startFilter() listModel {
	l.filtering = true
	l.filter.Focus()
	return l
}

func (l listModel) stopFilter() listModel {
	l.filtering = false
	l.filter.Blur()
	return l
}

func (l listModel) clearFilter() listModel {
	l.filter.SetValue("")
	l = l.stopFilter()
	return l.applyFilter()
}

func (l listModel) hasFilter() bool {
	return strings.TrimSpace(l.filter.Value()) != ""
}

func (l listModel) updateFilter(msg tea.Msg) (listModel, tea.Cmd) {
	var cmd tea.Cmd
	l.filter, cmd = l.filter.Update(msg)
	l.cursor = 0
	return l.applyFilter(), cmd
}

func (l listModel) move(delta int) listModel {
	if len(l.visible) == 0 {
		return l
	}
	l.cursor = min(max(l.cursor+delta, 0), len(l.visible)-1)
	return l
}

func (l listModel) focused() (vault.Env, bool) {
	if l.cursor < 0 || l.cursor >= len(l.visible) {
		return vault.Env{}, false
	}
	return l.envs[l.visible[l.cursor]], true
}

func (l listModel) hasEnv(name string) bool {
	return slices.ContainsFunc(l.envs, func(e vault.Env) bool { return e.Name == name })
}

func (l listModel) isMarked(name string) bool {
	return slices.Contains(l.marked, name)
}

func (l listModel) toggleMark() listModel {
	env, ok := l.focused()
	if !ok {
		return l
	}
	if l.isMarked(env.Name) {
		l.marked = slices.DeleteFunc(slices.Clone(l.marked), func(n string) bool { return n == env.Name })
		return l
	}
	l.marked = append(slices.Clone(l.marked), env.Name)
	return l
}

func (l listModel) withMarked(names []string) listModel {
	l.marked = slices.DeleteFunc(slices.Clone(names), func(name string) bool {
		return !l.hasEnv(name)
	})
	return l
}

func (l listModel) renameMarked(oldName, newName string) listModel {
	i := slices.Index(l.marked, oldName)
	if i < 0 {
		return l
	}
	l.marked = slices.Clone(l.marked)
	l.marked[i] = newName
	return l
}

func (l listModel) markPosition(name string) int {
	return slices.Index(l.marked, name) + 1
}

func (l listModel) markLabel(st styles, name string) string {
	width := max(lipgloss.Width(markOff), len(fmt.Sprintf("[%d]", len(l.marked))))
	pos := l.markPosition(name)
	if pos == 0 {
		return padRight(markOff, width)
	}
	return st.marked.Render(padRight(fmt.Sprintf("[%d]", pos), width))
}

func (l listModel) markedEnvs() []vault.Env {
	out := make([]vault.Env, 0, len(l.marked))
	for _, name := range l.marked {
		for _, env := range l.envs {
			if env.Name == name {
				out = append(out, env.Clone())
				break
			}
		}
	}
	return out
}

func (l listModel) view(st styles, width, height int) string {
	leftWidth := max(int(float64(width)*leftPaneRatio), minPaneWidth)
	rightWidth := max(width-leftWidth, minPaneWidth)
	innerHeight := max(height-paneBorder, 1)
	left := st.pane.Width(leftWidth - paneBorder).Height(innerHeight).Render(l.leftView(st, leftWidth-4, innerHeight))
	right := st.pane.Width(rightWidth - paneBorder).Height(innerHeight).Render(l.rightView(st, rightWidth-4, innerHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (l listModel) leftView(st styles, width, height int) string {
	lines := make([]string, 0, height)
	if l.filtering || l.hasFilter() {
		l.filter.Width = max(width-lipgloss.Width(l.filter.Prompt)-1, 1)
		lines = append(lines, l.filter.View())
	}
	switch {
	case l.failed:
		return strings.Join(append(lines, st.errorText.Render("não foi possível ler o cofre")), "\n")
	case !l.loaded:
		return strings.Join(append(lines, st.subtle.Render("carregando...")), "\n")
	case len(l.envs) == 0:
		return strings.Join(append(lines, st.subtle.Render("nenhuma env no cofre")), "\n")
	case len(l.visible) == 0:
		return strings.Join(append(lines, st.subtle.Render("nenhuma env corresponde ao filtro")), "\n")
	}
	start, end := scrollWindow(l.cursor, len(l.visible), height-len(lines))
	for i := start; i < end; i++ {
		lines = append(lines, l.row(st, l.envs[l.visible[i]], i == l.cursor, width))
	}
	return strings.Join(lines, "\n")
}

func (l listModel) row(st styles, env vault.Env, focused bool, width int) string {
	prefix := noCursor
	if focused {
		prefix = cursorMarker
	}
	mark := l.markLabel(st, env.Name)
	meta := fmt.Sprintf("%s  %s", keyCount(len(env.Vars)), formatTime(env))
	nameWidth := max(width-lipgloss.Width(prefix)-lipgloss.Width(mark)-lipgloss.Width(meta)-3, 4)
	name := padRight(truncate(env.Name, nameWidth), nameWidth)
	line := fmt.Sprintf("%s%s %s  %s", prefix, mark, name, st.subtle.Render(meta))
	if focused {
		return st.focused.Render(line)
	}
	return line
}

func (l listModel) rightView(st styles, width, height int) string {
	env, ok := l.focused()
	if !ok {
		return st.subtle.Render("selecione uma env")
	}
	return strings.Join(envSummary(st, env, width, height), "\n")
}

func envSummary(st styles, env vault.Env, width, height int) []string {
	lines := []string{
		st.title.Render(truncate(env.Name, width)),
		st.label.Render("descrição: ") + truncate(orDash(env.Description), max(width-11, 1)),
		st.label.Render("tags: ") + truncate(orDash(strings.Join(env.Tags, ", ")), max(width-6, 1)),
		st.label.Render(fmt.Sprintf("chaves (%d):", len(env.Vars))),
	}
	room := height - len(lines)
	for i, v := range env.Vars {
		if i >= room {
			break
		}
		if i == room-1 && len(env.Vars) > room {
			lines = append(lines, st.subtle.Render(fmt.Sprintf("  mais %d", len(env.Vars)-i)))
			break
		}
		keyWidth := max(width-lipgloss.Width(maskedValue)-3, 1)
		lines = append(lines, "  "+padRight(truncate(v.Key, keyWidth), keyWidth)+" "+st.masked.Render(maskedValue))
	}
	return lines
}

func keyCount(n int) string {
	if n == 1 {
		return "1 chave"
	}
	return fmt.Sprintf("%d chaves", n)
}

func formatTime(env vault.Env) string {
	if env.UpdatedAt.IsZero() {
		return "-"
	}
	return env.UpdatedAt.Local().Format(timeLayout)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return singleLine(s)
}
