package compose

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
)

const (
	targetPromptLabel = "destino: "
	keyColumn         = 24
	fromColumn        = 20
	writingStatus     = "gravando..."
	exportingStatus   = "exportando..."
)

var (
	errPlanPending = errors.New("aguarde a prévia da montagem")
	errNoTarget    = errors.New("informe o arquivo de destino")
)

type Deps struct {
	Template   *composeusecase.LoadTemplate
	Plan       *composeusecase.PlanLoad
	Load       *composeusecase.LoadEnvFile
	Export     *composeusecase.LoadShellExports
	ExportFile string
	Dialect    string
	Dir        string
}

func (d Deps) terminalReady() bool {
	return d.Export != nil && d.ExportFile != ""
}

func (d Deps) terminalHint() string {
	return "sem o wrapper de shell, só arquivo: adicione " + composeusecase.ShellInitLine(d.Dialect) + " ao rc do shell"
}

type Intent int

const (
	IntentNone Intent = iota
	IntentClose
	IntentHelp
	IntentReordered
	IntentStatus
)

type Effect struct {
	Intent  Intent
	Status  string
	Warning string
	Err     error
}

func statusEffect(status string) Effect {
	return Effect{Intent: IntentStatus, Status: status}
}

func errorEffect(err error) Effect {
	return Effect{Intent: IntentStatus, Err: err}
}

type dest int

const (
	destTerminal dest = iota
	destFile
)

type pane int

const (
	paneOrder pane = iota
	panePreview
	paneTarget
	paneCount
)

type PlanMsg struct {
	seq    int
	result composeusecase.PlanLoadResult
	tmpl   composeusecase.TemplateView
	Err    error
}

type ExportedMsg struct {
	Result composeusecase.LoadShellExportsResult
	Err    error
}

type WrittenMsg struct {
	Target string
	Result composeusecase.LoadEnvFileResult
	Err    error
}

type Model struct {
	ctx      context.Context
	keys     theme.KeyMap
	deps     Deps
	order    viewmodel.Selection
	cursor   int
	pane     pane
	row      int
	expanded map[string]bool
	target   textinput.Model
	dest     dest
	seq      int
	planned  bool
	plan     composeusecase.PlanView
	tmpl     composeusecase.TemplateView
}

func New(ctx context.Context, names []string, deps Deps) Model {
	ti := textinput.New()
	ti.Prompt = targetPromptLabel
	ti.SetValue(viewmodel.DefaultEnvFile)
	ti.CharLimit = 256
	ti.Cursor.SetMode(cursor.CursorStatic)
	target := destFile
	if deps.terminalReady() {
		target = destTerminal
	}
	return Model{
		ctx:      ctx,
		keys:     theme.DefaultKeyMap(),
		deps:     deps,
		order:    viewmodel.NewSelection(names),
		expanded: map[string]bool{},
		target:   ti,
		dest:     target,
	}
}

func (c Model) Order() []string {
	return c.order.Names()
}

func (c Model) Plan() composeusecase.PlanView {
	return c.plan
}

func (c Model) Template() composeusecase.TemplateView {
	return c.tmpl
}

func (c Model) Planned() bool {
	return c.planned
}

func (c Model) editingTarget() bool {
	return c.pane == paneTarget && c.dest == destFile
}

func (c Model) toggleDest() (Model, bool) {
	if !c.deps.terminalReady() {
		return c, false
	}
	if c.dest == destTerminal {
		c.dest = destFile
	} else {
		c.dest = destTerminal
	}
	return c.focusTarget(), true
}

func (c Model) focusTarget() Model {
	if c.editingTarget() {
		c.target.Focus()
	} else {
		c.target.Blur()
	}
	return c
}

func (c Model) targetValue() string {
	return strings.TrimSpace(c.target.Value())
}

func (c Model) PlanCmd() (Model, tea.Cmd) {
	c.seq++
	ctx, seq, names, deps := c.ctx, c.seq, c.order.Names(), c.deps
	return c, func() tea.Msg {
		tmpl, err := deps.Template.Execute(ctx, composeusecase.LoadTemplateInput{Dir: deps.Dir})
		if err != nil {
			return PlanMsg{seq: seq, Err: viewmodel.TemplateError(err)}
		}
		result, err := deps.Plan.Execute(ctx, composeusecase.PlanLoadInput{Envs: names, Template: tmpl, Target: viewmodel.DefaultEnvFile, Dir: deps.Dir})
		return PlanMsg{seq: seq, result: result, tmpl: tmpl, Err: err}
	}
}

func (c Model) OnPlan(msg PlanMsg) (updated Model, current bool) {
	if msg.seq != c.seq {
		return c, false
	}
	if msg.Err != nil {
		return c, true
	}
	c.plan = msg.result.Plan
	c.tmpl = msg.tmpl
	c.planned = true
	c.row = min(c.row, max(len(c.plan.Vars)-1, 0))
	return c, true
}

func (c Model) WriteCmd(existing composeusecase.ExistingTarget) (tea.Cmd, error) {
	if !c.planned {
		return nil, errPlanPending
	}
	target := c.targetValue()
	if target == "" {
		return nil, errNoTarget
	}
	ctx, names, deps, tmpl := c.ctx, c.order.Names(), c.deps, c.tmpl
	return func() tea.Msg {
		result, err := deps.Load.Execute(ctx, composeusecase.LoadEnvFileInput{
			Envs:     names,
			Template: tmpl,
			Target:   target,
			Dir:      deps.Dir,
			Existing: existing,
		})
		return WrittenMsg{Target: target, Result: result, Err: err}
	}, nil
}

func (c Model) exportCmd() (tea.Cmd, error) {
	if !c.planned {
		return nil, errPlanPending
	}
	ctx, names, deps, tmpl := c.ctx, c.order.Names(), c.deps, c.tmpl
	return func() tea.Msg {
		result, err := deps.Export.Execute(ctx, composeusecase.LoadShellExportsInput{
			Envs:       names,
			Template:   tmpl,
			Dialect:    deps.Dialect,
			ExportFile: deps.ExportFile,
		})
		return ExportedMsg{Result: result, Err: err}
	}, nil
}

func (c Model) Update(msg tea.KeyMsg) (Model, tea.Cmd, Effect) {
	if key.Matches(msg, c.keys.Back) || msg.Type == tea.KeyCtrlC {
		return c, nil, Effect{Intent: IntentClose}
	}
	if c.editingTarget() {
		return c.updateTarget(msg)
	}
	switch {
	case key.Matches(msg, c.keys.Quit):
		return c, nil, Effect{Intent: IntentClose}
	case key.Matches(msg, c.keys.Write):
		return c.confirm()
	case key.Matches(msg, c.keys.Target):
		return c.switchDest()
	case key.Matches(msg, c.keys.NextPane):
		c = c.cyclePane(1)
	case key.Matches(msg, c.keys.PrevPane):
		c = c.cyclePane(-1)
	case key.Matches(msg, c.keys.MoveUp):
		return c.reorder(-1)
	case key.Matches(msg, c.keys.MoveDown):
		return c.reorder(1)
	case key.Matches(msg, c.keys.Up):
		c = c.moveCursor(-1)
	case key.Matches(msg, c.keys.Down):
		c = c.moveCursor(1)
	case key.Matches(msg, c.keys.Expand):
		c = c.toggleExpanded()
	case key.Matches(msg, c.keys.Help):
		return c, nil, Effect{Intent: IntentHelp}
	}
	return c, nil, Effect{}
}

func (c Model) updateTarget(msg tea.KeyMsg) (Model, tea.Cmd, Effect) {
	switch {
	case key.Matches(msg, c.keys.Write):
		return c.write(composeusecase.RefuseExisting)
	case key.Matches(msg, c.keys.NextPane):
		return c.cyclePane(1), nil, Effect{}
	case key.Matches(msg, c.keys.PrevPane):
		return c.cyclePane(-1), nil, Effect{}
	}
	var cmd tea.Cmd
	c.target, cmd = c.target.Update(msg)
	return c, cmd, Effect{}
}

func (c Model) write(existing composeusecase.ExistingTarget) (Model, tea.Cmd, Effect) {
	cmd, err := c.WriteCmd(existing)
	if err != nil {
		return c, nil, errorEffect(err)
	}
	return c, cmd, statusEffect(writingStatus)
}

func (c Model) confirm() (Model, tea.Cmd, Effect) {
	if c.dest == destFile {
		return c.write(composeusecase.RefuseExisting)
	}
	cmd, err := c.exportCmd()
	if err != nil {
		return c, nil, errorEffect(err)
	}
	return c, cmd, statusEffect(exportingStatus)
}

func (c Model) switchDest() (Model, tea.Cmd, Effect) {
	toggled, ok := c.toggleDest()
	if !ok {
		return c, nil, Effect{Intent: IntentStatus, Warning: c.deps.terminalHint()}
	}
	return toggled, nil, statusEffect("")
}

func (c Model) reorder(delta int) (Model, tea.Cmd, Effect) {
	if c.pane != paneOrder {
		return c, nil, Effect{}
	}
	order, moved := c.order.Move(c.cursor, delta)
	if !moved {
		return c, nil, Effect{}
	}
	c.order = order
	c.cursor += delta
	c, cmd := c.PlanCmd()
	return c, cmd, Effect{Intent: IntentReordered}
}

func (c Model) moveCursor(delta int) Model {
	switch c.pane {
	case paneOrder:
		c.cursor = min(max(c.cursor+delta, 0), max(c.order.Len()-1, 0))
	case panePreview:
		c.row = min(max(c.row+delta, 0), max(len(c.plan.Vars)-1, 0))
	}
	return c
}

func (c Model) cyclePane(delta int) Model {
	c.pane = (c.pane + pane(delta) + paneCount) % paneCount
	return c.focusTarget()
}

func (c Model) toggleExpanded() Model {
	if c.pane != panePreview || c.row >= len(c.plan.Vars) {
		return c
	}
	resolved := c.plan.Vars[c.row]
	if len(resolved.Shadows) == 0 {
		return c
	}
	expanded := make(map[string]bool, len(c.expanded)+1)
	for k, v := range c.expanded {
		expanded[k] = v
	}
	expanded[resolved.Key] = !expanded[resolved.Key]
	c.expanded = expanded
	return c
}

func (c Model) View(st theme.Styles, width, height int) string {
	leftWidth := max(int(float64(width)*theme.LeftPaneRatio), theme.MinPaneWidth)
	rightWidth := max(width-leftWidth, theme.MinPaneWidth)
	innerHeight := max(height-theme.PaneBorder, 1)
	left := c.paneStyle(st, paneOrder, paneTarget).Width(leftWidth - theme.PaneBorder).Height(innerHeight).Render(c.leftView(st, leftWidth-4))
	right := c.paneStyle(st, panePreview).Width(rightWidth - theme.PaneBorder).Height(innerHeight).Render(c.rightView(st, rightWidth-4, innerHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (c Model) paneStyle(st theme.Styles, panes ...pane) lipgloss.Style {
	for _, p := range panes {
		if p == c.pane {
			return st.PaneActive
		}
	}
	return st.Pane
}

func (c Model) leftView(st theme.Styles, width int) string {
	lines := []string{
		st.Title.Render("Montagem"),
		st.Subtle.Render("a de baixo vence"),
		"",
	}
	for i, item := range c.order.Numbered() {
		focused := i == c.cursor && c.pane == paneOrder
		prefix := theme.NoCursor
		if focused {
			prefix = theme.CursorMarker
		}
		line := theme.Truncate(prefix+item, width)
		if focused {
			line = st.Focused.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, c.destLines(st, width)...)
	return strings.Join(lines, "\n")
}

func (c Model) destLines(st theme.Styles, width int) []string {
	if c.dest == destTerminal {
		line := theme.Truncate(targetPromptLabel+"terminal atual", width)
		if c.pane == paneTarget {
			line = st.Focused.Render(line)
		}
		return []string{line, st.Subtle.Render(theme.Truncate("t: gravar em arquivo", width))}
	}
	c.target.Width = max(width-lipgloss.Width(c.target.Prompt)-1, 1)
	if c.deps.terminalReady() {
		return []string{c.target.View(), st.Subtle.Render(theme.Truncate("t: exportar no terminal", width))}
	}
	return []string{
		c.target.View(),
		st.WarnText.Render(theme.Truncate("sem wrapper: só arquivo", width)),
		st.Subtle.Render(theme.Truncate(composeusecase.ShellInitLine(c.deps.Dialect), width)),
	}
}

func (c Model) rightView(st theme.Styles, width, height int) string {
	if !c.planned {
		return st.Subtle.Render("montando prévia...")
	}
	lines := []string{st.Title.Render("prévia: " + strings.Join(c.plan.Envs, ", "))}
	rows := viewmodel.PreviewRows(c.plan, c.expanded)
	focusLine := 0
	if len(rows) == 0 {
		lines = append(lines, st.Subtle.Render("nenhuma chave"))
	}
	for i, row := range rows {
		if i == c.row {
			focusLine = len(lines)
		}
		lines = append(lines, previewLine(st, row, i == c.row && c.pane == panePreview, width))
		for _, shadow := range row.Shadows {
			lines = append(lines, st.Subtle.Render(theme.Truncate("      "+shadow, width)))
		}
	}
	lines = append(lines, c.templateLines(st, width)...)
	start, end := theme.ScrollWindow(focusLine, len(lines), height)
	return strings.Join(lines[start:end], "\n")
}

func previewLine(st theme.Styles, row viewmodel.PreviewRow, focused bool, width int) string {
	prefix := theme.NoCursor
	if focused {
		prefix = theme.CursorMarker
	}
	line := prefix + theme.PadRight(theme.Truncate(row.Key, keyColumn), keyColumn) + " " + theme.PadRight(theme.Truncate(row.From, fromColumn), fromColumn)
	if row.Conflicted() {
		line += " " + row.Conflict
	}
	line = theme.Truncate(line, width)
	switch {
	case focused:
		return st.Focused.Render(line)
	case row.Conflicted():
		return st.WarnText.Render(line)
	}
	return line
}

func (c Model) templateLines(st theme.Styles, width int) []string {
	checklist, found := viewmodel.TemplateChecklist(c.tmpl, c.plan)
	if !found {
		return nil
	}
	lines := []string{"", st.Label.Render(checklist.Header)}
	for _, item := range checklist.Items {
		text := theme.Truncate(item.Text, width)
		if item.Missing {
			text = st.ErrorText.Render(text)
		}
		lines = append(lines, text)
	}
	if checklist.Extra != "" {
		lines = append(lines, st.Subtle.Render(theme.Truncate(checklist.Extra, width)))
	}
	return lines
}
