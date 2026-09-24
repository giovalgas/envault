package list

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

type Intent int

const (
	IntentNone Intent = iota
	IntentQuit
	IntentHelp
	IntentOpen
	IntentDuplicate
	IntentRename
	IntentDelete
	IntentNew
	IntentEdit
	IntentImport
	IntentCompose
)

type Model struct {
	keys      theme.KeyMap
	envs      []vaultusecase.EnvView
	visible   []int
	cursor    int
	selection viewmodel.Selection
	filter    textinput.Model
	filtering bool
	loaded    bool
	failed    bool
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "nome, descrição, tag ou chave"
	ti.CharLimit = 64
	ti.Cursor.SetMode(cursor.CursorStatic)
	return Model{keys: theme.DefaultKeyMap(), filter: ti}
}

func (l Model) SetEnvs(envs []vaultusecase.EnvView, focus string) Model {
	if focus == "" {
		if current, ok := l.Focused(); ok {
			focus = current.Name
		}
	}
	l.envs = envs
	l.loaded = true
	l.failed = false
	l.selection = l.selection.Keep(l.HasEnv)
	l = l.applyFilter()
	l = l.focusName(focus)
	return l
}

func (l Model) MarkFailed() Model {
	l.failed = true
	return l
}

func (l Model) Len() int {
	return len(l.envs)
}

func (l Model) Envs() []vaultusecase.EnvView {
	return l.envs
}

func (l Model) VisibleNames() []string {
	names := make([]string, 0, len(l.visible))
	for _, idx := range l.visible {
		names = append(names, l.envs[idx].Name)
	}
	return names
}

func (l Model) Filtering() bool {
	return l.filtering
}

func (l Model) HasFilter() bool {
	return strings.TrimSpace(l.filter.Value()) != ""
}

func (l Model) Focused() (vaultusecase.EnvView, bool) {
	if l.cursor < 0 || l.cursor >= len(l.visible) {
		return vaultusecase.EnvView{}, false
	}
	return l.envs[l.visible[l.cursor]], true
}

func (l Model) HasEnv(name string) bool {
	return slices.ContainsFunc(l.envs, func(e vaultusecase.EnvView) bool { return e.Name == name })
}

func (l Model) Marked() []string {
	return l.selection.Names()
}

func (l Model) MarkedEnvs() []vaultusecase.EnvView {
	return l.selection.Pick(l.envs)
}

func (l Model) WithMarked(names []string) Model {
	l.selection = viewmodel.NewSelection(names).Keep(l.HasEnv)
	return l
}

func (l Model) RenameMarked(oldName, newName string) Model {
	l.selection = l.selection.Rename(oldName, newName)
	return l
}

func (l Model) Update(msg tea.KeyMsg) (Model, tea.Cmd, Intent) {
	if l.filtering {
		switch {
		case key.Matches(msg, l.keys.Back):
			return l.clearFilter(), nil, IntentNone
		case key.Matches(msg, l.keys.Confirm):
			return l.stopFilter(), nil, IntentNone
		}
		l, cmd := l.updateFilter(msg)
		return l, cmd, IntentNone
	}
	switch {
	case key.Matches(msg, l.keys.Quit):
		return l, nil, IntentQuit
	case key.Matches(msg, l.keys.Back):
		if l.HasFilter() {
			l = l.clearFilter()
		}
	case key.Matches(msg, l.keys.Up):
		l = l.move(-1)
	case key.Matches(msg, l.keys.Down):
		l = l.move(1)
	case key.Matches(msg, l.keys.Mark):
		l = l.toggleMark()
	case key.Matches(msg, l.keys.Filter):
		l = l.startFilter()
	case key.Matches(msg, l.keys.Help):
		return l, nil, IntentHelp
	default:
		return l, nil, l.intentFor(msg)
	}
	return l, nil, IntentNone
}

func (l Model) intentFor(msg tea.KeyMsg) Intent {
	switch {
	case key.Matches(msg, l.keys.Open):
		return IntentOpen
	case key.Matches(msg, l.keys.Duplicate):
		return IntentDuplicate
	case key.Matches(msg, l.keys.Rename):
		return IntentRename
	case key.Matches(msg, l.keys.Delete):
		return IntentDelete
	case key.Matches(msg, l.keys.New):
		return IntentNew
	case key.Matches(msg, l.keys.Edit):
		return IntentEdit
	case key.Matches(msg, l.keys.Import):
		return IntentImport
	case key.Matches(msg, l.keys.Compose):
		return IntentCompose
	}
	return IntentNone
}

func (l Model) focusName(name string) Model {
	for i, idx := range l.visible {
		if l.envs[idx].Name == name {
			l.cursor = i
			return l
		}
	}
	l.cursor = min(l.cursor, max(len(l.visible)-1, 0))
	return l
}

func (l Model) applyFilter() Model {
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

func matchesFilter(env vaultusecase.EnvView, query string) bool {
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

func (l Model) startFilter() Model {
	l.filtering = true
	l.filter.Focus()
	return l
}

func (l Model) stopFilter() Model {
	l.filtering = false
	l.filter.Blur()
	return l
}

func (l Model) clearFilter() Model {
	l.filter.SetValue("")
	l = l.stopFilter()
	return l.applyFilter()
}

func (l Model) updateFilter(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	l.filter, cmd = l.filter.Update(msg)
	l.cursor = 0
	return l.applyFilter(), cmd
}

func (l Model) move(delta int) Model {
	if len(l.visible) == 0 {
		return l
	}
	l.cursor = min(max(l.cursor+delta, 0), len(l.visible)-1)
	return l
}

func (l Model) toggleMark() Model {
	env, ok := l.Focused()
	if !ok {
		return l
	}
	l.selection = l.selection.Toggle(env.Name)
	return l
}

func (l Model) SelectionView(st theme.Styles, width int) string {
	text, empty := l.selection.Panel()
	if empty {
		return st.Subtle.Render(theme.Truncate(text, width))
	}
	return st.Label.Render(theme.Truncate(text, width))
}

func (l Model) View(st theme.Styles, width, height int) string {
	leftWidth := max(int(float64(width)*theme.LeftPaneRatio), theme.MinPaneWidth)
	rightWidth := max(width-leftWidth, theme.MinPaneWidth)
	innerHeight := max(height-theme.PaneBorder, 1)
	left := st.Pane.Width(leftWidth - theme.PaneBorder).Height(innerHeight).Render(l.leftView(st, leftWidth-4, innerHeight))
	right := st.Pane.Width(rightWidth - theme.PaneBorder).Height(innerHeight).Render(l.rightView(st, rightWidth-4, innerHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (l Model) leftView(st theme.Styles, width, height int) string {
	lines := make([]string, 0, height)
	if l.filtering || l.HasFilter() {
		l.filter.Width = max(width-lipgloss.Width(l.filter.Prompt)-1, 1)
		lines = append(lines, l.filter.View())
	}
	switch {
	case l.failed:
		return strings.Join(append(lines, st.ErrorText.Render("não foi possível ler o cofre")), "\n")
	case !l.loaded:
		return strings.Join(append(lines, st.Subtle.Render("carregando...")), "\n")
	case len(l.envs) == 0:
		return strings.Join(append(lines, st.Subtle.Render("nenhuma env no cofre")), "\n")
	case len(l.visible) == 0:
		return strings.Join(append(lines, st.Subtle.Render("nenhuma env corresponde ao filtro")), "\n")
	}
	start, end := theme.ScrollWindow(l.cursor, len(l.visible), height-len(lines))
	for i := start; i < end; i++ {
		lines = append(lines, l.row(st, l.envs[l.visible[i]], i == l.cursor, width))
	}
	return strings.Join(lines, "\n")
}

func (l Model) row(st theme.Styles, env vaultusecase.EnvView, focused bool, width int) string {
	prefix := theme.NoCursor
	if focused {
		prefix = theme.CursorMarker
	}
	mark, marked := l.selection.MarkLabel(env.Name)
	if marked {
		mark = st.Marked.Render(mark)
	}
	meta := viewmodel.ListMeta(env)
	nameWidth := max(width-lipgloss.Width(prefix)-lipgloss.Width(mark)-lipgloss.Width(meta)-3, 4)
	name := theme.PadRight(theme.Truncate(env.Name, nameWidth), nameWidth)
	line := fmt.Sprintf("%s%s %s  %s", prefix, mark, name, st.Subtle.Render(meta))
	if focused {
		return st.Focused.Render(line)
	}
	return line
}

func (l Model) rightView(st theme.Styles, width, height int) string {
	env, ok := l.Focused()
	if !ok {
		return st.Subtle.Render("selecione uma env")
	}
	return strings.Join(envSummary(st, env, width, height), "\n")
}

func envSummary(st theme.Styles, env vaultusecase.EnvView, width, height int) []string {
	header := viewmodel.Header(env)
	rows := viewmodel.MaskedRows(env)
	lines := []string{
		st.Title.Render(theme.Truncate(header.Name, width)),
		st.Label.Render("descrição: ") + theme.Truncate(header.Description, max(width-11, 1)),
		st.Label.Render("tags: ") + theme.Truncate(header.Tags, max(width-6, 1)),
		st.Label.Render(fmt.Sprintf("chaves (%d):", len(rows))),
	}
	room := height - len(lines)
	for i, row := range rows {
		if i >= room {
			break
		}
		if i == room-1 && len(rows) > room {
			lines = append(lines, st.Subtle.Render(fmt.Sprintf("  mais %d", len(rows)-i)))
			break
		}
		keyWidth := max(width-lipgloss.Width(row.Value)-3, 1)
		lines = append(lines, "  "+theme.PadRight(theme.Truncate(row.Key, keyWidth), keyWidth)+" "+st.Masked.Render(row.Value))
	}
	return lines
}
