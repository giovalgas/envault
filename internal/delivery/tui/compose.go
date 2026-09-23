package tui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	composedomain "github.com/giovalgas/envault/internal/compose/domain"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/dotenv"
)

const (
	templateFileName  = ".env.example"
	targetPromptLabel = "destino: "
	composeKeyColumn  = 24
	composeFromColumn = 20
)

type composeDeps struct {
	plan       *composeusecase.PlanLoad
	load       *composeusecase.LoadEnvFile
	export     *composeusecase.LoadShellExports
	exportFile string
	dialect    string
	dir        string
}

func (d composeDeps) terminalReady() bool {
	return d.export != nil && d.exportFile != ""
}

func (d composeDeps) terminalHint() string {
	return "sem o wrapper de shell, só arquivo: adicione " + composeusecase.ShellInitLine(d.dialect) + " ao rc do shell"
}

type composeDest int

const (
	destTerminal composeDest = iota
	destFile
)

type composePane int

const (
	paneOrder composePane = iota
	panePreview
	paneTarget
	paneCount
)

type templateInfo struct {
	path     string
	found    bool
	template *composedomain.Template
}

type composeModel struct {
	deps     composeDeps
	order    []string
	cursor   int
	pane     composePane
	row      int
	expanded map[string]bool
	target   textinput.Model
	dest     composeDest
	seq      int
	planned  bool
	plan     composedomain.Plan
	tmpl     templateInfo
}

type composePlanMsg struct {
	seq    int
	result composeusecase.PlanLoadResult
	tmpl   templateInfo
	err    error
}

type composeExportedMsg struct {
	result composeusecase.LoadShellExportsResult
	err    error
}

type composeWrittenMsg struct {
	target string
	result composeusecase.LoadEnvFileResult
	err    error
}

func newComposeModel(names []string, deps composeDeps) composeModel {
	ti := textinput.New()
	ti.Prompt = targetPromptLabel
	ti.SetValue(defaultEnvFile)
	ti.CharLimit = 256
	ti.Cursor.SetMode(cursor.CursorStatic)
	dest := destFile
	if deps.terminalReady() {
		dest = destTerminal
	}
	return composeModel{
		deps:     deps,
		order:    slices.Clone(names),
		expanded: map[string]bool{},
		target:   ti,
		dest:     dest,
	}
}

func (c composeModel) editingTarget() bool {
	return c.pane == paneTarget && c.dest == destFile
}

func (c composeModel) toggleDest() (composeModel, bool) {
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

func (c composeModel) focusTarget() composeModel {
	if c.editingTarget() {
		c.target.Focus()
	} else {
		c.target.Blur()
	}
	return c
}

func (c composeModel) targetValue() string {
	return strings.TrimSpace(c.target.Value())
}

func (c composeModel) planCmd(ctx context.Context) (composeModel, tea.Cmd) {
	c.seq++
	seq, names, deps := c.seq, slices.Clone(c.order), c.deps
	target := resolvePath(deps.dir, defaultEnvFile)
	return c, func() tea.Msg {
		tmpl, err := loadTemplate(deps.dir)
		if err != nil {
			return composePlanMsg{seq: seq, err: err}
		}
		result, err := deps.plan.Execute(ctx, composeusecase.PlanLoadInput{Envs: names, Template: tmpl.template, Target: target})
		return composePlanMsg{seq: seq, result: result, tmpl: tmpl, err: err}
	}
}

func (c composeModel) onPlan(msg composePlanMsg) (composeModel, bool) {
	if msg.seq != c.seq {
		return c, false
	}
	if msg.err != nil {
		return c, true
	}
	c.plan = msg.result.Plan
	c.tmpl = msg.tmpl
	c.planned = true
	c.row = min(c.row, max(len(c.plan.Vars)-1, 0))
	return c, true
}

func (c composeModel) writeCmd(ctx context.Context, existing composeusecase.ExistingTarget) (tea.Cmd, error) {
	if !c.planned {
		return nil, errors.New("aguarde a prévia da montagem")
	}
	target := c.targetValue()
	if target == "" {
		return nil, errors.New("informe o arquivo de destino")
	}
	names, deps, tmpl := slices.Clone(c.order), c.deps, c.tmpl
	return func() tea.Msg {
		result, err := deps.load.Execute(ctx, composeusecase.LoadEnvFileInput{
			Envs:     names,
			Template: tmpl.template,
			Target:   resolvePath(deps.dir, target),
			Existing: existing,
		})
		return composeWrittenMsg{target: target, result: result, err: err}
	}, nil
}

func (c composeModel) exportCmd(ctx context.Context) (tea.Cmd, error) {
	if !c.planned {
		return nil, errors.New("aguarde a prévia da montagem")
	}
	names, deps, tmpl := slices.Clone(c.order), c.deps, c.tmpl
	return func() tea.Msg {
		result, err := deps.export.Execute(ctx, composeusecase.LoadShellExportsInput{
			Envs:       names,
			Template:   tmpl.template,
			Dialect:    deps.dialect,
			ExportFile: deps.exportFile,
		})
		return composeExportedMsg{result: result, err: err}
	}, nil
}

func (c composeModel) moveCursor(delta int) composeModel {
	switch c.pane {
	case paneOrder:
		c.cursor = min(max(c.cursor+delta, 0), max(len(c.order)-1, 0))
	case panePreview:
		c.row = min(max(c.row+delta, 0), max(len(c.plan.Vars)-1, 0))
	}
	return c
}

func (c composeModel) reorder(delta int) (composeModel, bool) {
	next := c.cursor + delta
	if c.pane != paneOrder || next < 0 || next >= len(c.order) {
		return c, false
	}
	c.order = slices.Clone(c.order)
	c.order[c.cursor], c.order[next] = c.order[next], c.order[c.cursor]
	c.cursor = next
	return c, true
}

func (c composeModel) cyclePane(delta int) composeModel {
	c.pane = (c.pane + composePane(delta) + paneCount) % paneCount
	return c.focusTarget()
}

func (c composeModel) toggleExpanded() composeModel {
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

func (m Model) openCompose(msg composeOpenMsg) (tea.Model, tea.Cmd) {
	compose, cmd := newComposeModel(msg.names, msg.deps).planCmd(m.ctx)
	m.compose = compose
	m.screen = screenCompose
	m = m.setStatus("", nil)
	return m, cmd
}

func (m Model) onComposePlan(msg composePlanMsg) Model {
	if m.screen != screenCompose {
		return m
	}
	compose, current := m.compose.onPlan(msg)
	m.compose = compose
	if current && msg.err != nil {
		m.logger.Printf("montar prévia: %v", msg.err)
		return m.setStatus("", fmt.Errorf("montar prévia: %w", msg.err))
	}
	return m
}

func (m Model) closeCompose() Model {
	m.screen = screenList
	m.compose = composeModel{}
	return m
}

func (m Model) updateCompose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.compose
	if key.Matches(msg, m.keys.Back) || msg.Type == tea.KeyCtrlC {
		return m.closeCompose(), nil
	}
	if c.editingTarget() {
		switch {
		case key.Matches(msg, m.keys.Write):
			return m.writeCompose(composeusecase.RefuseExisting)
		case key.Matches(msg, m.keys.NextPane):
			m.compose = c.cyclePane(1)
		case key.Matches(msg, m.keys.PrevPane):
			m.compose = c.cyclePane(-1)
		default:
			var cmd tea.Cmd
			m.compose.target, cmd = c.target.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m.closeCompose(), nil
	case key.Matches(msg, m.keys.Write):
		return m.confirmCompose()
	case key.Matches(msg, m.keys.Target):
		return m.toggleComposeDest(), nil
	case key.Matches(msg, m.keys.NextPane):
		m.compose = c.cyclePane(1)
	case key.Matches(msg, m.keys.PrevPane):
		m.compose = c.cyclePane(-1)
	case key.Matches(msg, m.keys.MoveUp):
		return m.reorderCompose(-1)
	case key.Matches(msg, m.keys.MoveDown):
		return m.reorderCompose(1)
	case key.Matches(msg, m.keys.Up):
		m.compose = c.moveCursor(-1)
	case key.Matches(msg, m.keys.Down):
		m.compose = c.moveCursor(1)
	case key.Matches(msg, m.keys.Expand):
		m.compose = c.toggleExpanded()
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
	}
	return m, nil
}

func (m Model) reorderCompose(delta int) (tea.Model, tea.Cmd) {
	compose, moved := m.compose.reorder(delta)
	if !moved {
		return m, nil
	}
	var plan, save tea.Cmd
	m.compose, plan = compose.planCmd(m.ctx)
	before := m.list.marked
	m.list = m.list.withMarked(m.compose.order)
	m, save = m.persistSelection(before)
	return m, tea.Batch(plan, save)
}

func (m Model) confirmCompose() (tea.Model, tea.Cmd) {
	if m.compose.dest == destFile {
		return m.writeCompose(composeusecase.RefuseExisting)
	}
	cmd, err := m.compose.exportCmd(m.ctx)
	if err != nil {
		return m.setStatus("", err), nil
	}
	return m.setStatus("exportando...", nil), cmd
}

func (m Model) toggleComposeDest() Model {
	compose, toggled := m.compose.toggleDest()
	if !toggled {
		return m.setWarning(m.compose.deps.terminalHint())
	}
	m.compose = compose
	return m.setStatus("", nil)
}

func (m Model) onComposeExported(msg composeExportedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.logger.Printf("exportar montagem: %v", msg.err)
		return m.setStatus("", fmt.Errorf("exportar no terminal: %w", msg.err)), nil
	}
	m.exported = exportedText(msg.result)
	return m, m.quitCmd()
}

func exportedText(result composeusecase.LoadShellExportsResult) string {
	text := fmt.Sprintf("%d variáveis de %s exportadas no terminal", result.Written, strings.Join(result.Plan.Envs, ", "))
	if missing := result.Plan.Missing; len(missing) > 0 {
		text += "; sem valor no template: " + strings.Join(missing, ", ")
	}
	return text
}

func (m Model) writeCompose(existing composeusecase.ExistingTarget) (tea.Model, tea.Cmd) {
	cmd, err := m.compose.writeCmd(m.ctx, existing)
	if err != nil {
		return m.setStatus("", err), nil
	}
	return m.setStatus("gravando...", nil), cmd
}

func (m Model) onComposeWritten(msg composeWrittenMsg) (tea.Model, tea.Cmd) {
	switch {
	case errors.Is(msg.err, composeusecase.ErrTargetExists):
		return m.openTargetExists(msg.target), nil
	case errors.Is(msg.err, composeusecase.ErrTargetIsDirectory):
		return m.setStatus("", fmt.Errorf("destino %s é um diretório", msg.target)), nil
	case msg.err != nil:
		m.logger.Printf("gravar montagem: %v", msg.err)
		return m.setStatus("", fmt.Errorf("gravar %s: %w", msg.target, msg.err)), nil
	}
	m = m.closeCompose()
	return m.onResult(ResultMsg{Status: writtenText(msg), Warning: writtenWarning(msg)})
}

func (m Model) openTargetExists(target string) Model {
	ctx, compose := m.ctx, m.compose
	write := func(existing composeusecase.ExistingTarget) func(string) (tea.Cmd, error) {
		return func(string) (tea.Cmd, error) { return compose.writeCmd(ctx, existing) }
	}
	modal := newConfirm("Destino já existe",
		fmt.Sprintf("%s já existe. Sobrescrever troca o arquivo inteiro; mesclar mantém as chaves locais e atualiza as das envs.", target)).
		withChoices(
			choice{label: "sobrescrever", run: write(composeusecase.ReplaceExisting)},
			choice{label: "mesclar", run: write(composeusecase.MergeExisting)},
			cancelChoice(),
		)
	m.modal = &modal
	return m.setStatus("", nil)
}

func writtenText(msg composeWrittenMsg) string {
	verb := "gravado"
	switch msg.result.Mode {
	case composeusecase.LoadOverwritten:
		verb = "sobrescrito"
	case composeusecase.LoadMerged:
		verb = "mesclado"
	}
	return fmt.Sprintf("%s %s com %d variáveis de %s", msg.target, verb, msg.result.Written, strings.Join(msg.result.Plan.Envs, ", "))
}

func writtenWarning(msg composeWrittenMsg) string {
	var warnings []string
	if msg.result.Target.Gitignored == composedomain.GitignoreNotIgnored {
		warnings = append(warnings, fmt.Sprintf("aviso: %s não está coberto pelo .gitignore", msg.target))
	}
	if missing := msg.result.Plan.Missing; len(missing) > 0 {
		warnings = append(warnings, "sem valor no template: "+strings.Join(missing, ", "))
	}
	return strings.Join(warnings, "; ")
}

func loadTemplate(dir string) (templateInfo, error) {
	path := resolvePath(dir, templateFileName)
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return templateInfo{path: templateFileName}, nil
	}
	if err != nil {
		return templateInfo{}, fmt.Errorf("ler %s: %w", templateFileName, err)
	}
	parsed, err := dotenv.ParseTemplate(data)
	if err != nil {
		return templateInfo{}, fmt.Errorf("%s: %w", templateFileName, err)
	}
	entries := make([]composedomain.TemplateEntry, len(parsed.Entries))
	for i, entry := range parsed.Entries {
		entries[i] = composedomain.TemplateEntry{Key: entry.Key, Default: entry.Default, HasDefault: entry.HasDefault}
	}
	return templateInfo{path: templateFileName, found: true, template: &composedomain.Template{Entries: entries}}, nil
}

func (c composeModel) view(st styles, width, height int) string {
	leftWidth := max(int(float64(width)*leftPaneRatio), minPaneWidth)
	rightWidth := max(width-leftWidth, minPaneWidth)
	innerHeight := max(height-paneBorder, 1)
	left := c.paneStyle(st, paneOrder, paneTarget).Width(leftWidth - paneBorder).Height(innerHeight).Render(c.leftView(st, leftWidth-4))
	right := c.paneStyle(st, panePreview).Width(rightWidth - paneBorder).Height(innerHeight).Render(c.rightView(st, rightWidth-4, innerHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (c composeModel) paneStyle(st styles, panes ...composePane) lipgloss.Style {
	if slices.Contains(panes, c.pane) {
		return st.pane.BorderForeground(colorAccent)
	}
	return st.pane
}

func (c composeModel) leftView(st styles, width int) string {
	lines := []string{
		st.title.Render("Montagem"),
		st.subtle.Render("a de baixo vence"),
		"",
	}
	for i, name := range c.order {
		prefix := noCursor
		if i == c.cursor && c.pane == paneOrder {
			prefix = cursorMarker
		}
		line := truncate(fmt.Sprintf("%s%d. %s", prefix, i+1, name), width)
		if i == c.cursor && c.pane == paneOrder {
			line = st.focused.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, c.destLines(st, width)...)
	return strings.Join(lines, "\n")
}

func (c composeModel) destLines(st styles, width int) []string {
	if c.dest == destTerminal {
		line := truncate(targetPromptLabel+"terminal atual", width)
		if c.pane == paneTarget {
			line = st.focused.Render(line)
		}
		return []string{line, st.subtle.Render(truncate("t: gravar em arquivo", width))}
	}
	c.target.Width = max(width-lipgloss.Width(c.target.Prompt)-1, 1)
	if c.deps.terminalReady() {
		return []string{c.target.View(), st.subtle.Render(truncate("t: exportar no terminal", width))}
	}
	return []string{
		c.target.View(),
		st.warnText.Render(truncate("sem wrapper: só arquivo", width)),
		st.subtle.Render(truncate(composeusecase.ShellInitLine(c.deps.dialect), width)),
	}
}

func (c composeModel) rightView(st styles, width, height int) string {
	if !c.planned {
		return st.subtle.Render("montando prévia...")
	}
	lines := []string{st.title.Render("prévia: " + strings.Join(c.plan.Envs, ", "))}
	focusLine := 0
	if len(c.plan.Vars) == 0 {
		lines = append(lines, st.subtle.Render("nenhuma chave"))
	}
	for i, resolved := range c.plan.Vars {
		if i == c.row {
			focusLine = len(lines)
		}
		lines = append(lines, c.previewRow(st, resolved, i == c.row && c.pane == panePreview, width))
		if c.expanded[resolved.Key] {
			for _, shadow := range resolved.ShadowNames() {
				lines = append(lines, st.subtle.Render(truncate("      sombreada em "+shadow, width)))
			}
		}
	}
	lines = append(lines, c.templateLines(st, width)...)
	start, end := scrollWindow(focusLine, len(lines), height)
	return strings.Join(lines[start:end], "\n")
}

func (c composeModel) previewRow(st styles, resolved composedomain.Resolved, focused bool, width int) string {
	prefix := noCursor
	if focused {
		prefix = cursorMarker
	}
	from := resolved.From
	if resolved.Default {
		from = "padrão do template"
	}
	line := prefix + padRight(truncate(resolved.Key, composeKeyColumn), composeKeyColumn) + " " + padRight(truncate(from, composeFromColumn), composeFromColumn)
	if n := len(resolved.Shadows); n > 0 {
		marker := "+"
		if c.expanded[resolved.Key] {
			marker = "-"
		}
		line += fmt.Sprintf(" conflito %s%d", marker, n)
	}
	line = truncate(line, width)
	switch {
	case focused:
		return st.focused.Render(line)
	case len(resolved.Shadows) > 0:
		return st.warnText.Render(line)
	}
	return line
}

func (c composeModel) templateLines(st styles, width int) []string {
	if !c.tmpl.found || c.tmpl.template == nil {
		return nil
	}
	missing := c.plan.Missing
	keys := c.tmpl.template.Keys()
	filled := len(keys) - len(missing)
	lines := []string{"", st.label.Render(fmt.Sprintf("template %s: %d de %d preenchidas", c.tmpl.path, filled, len(keys)))}
	defaults := make(map[string]bool, len(c.plan.Vars))
	for _, resolved := range c.plan.Vars {
		defaults[resolved.Key] = resolved.Default
	}
	for _, k := range keys {
		switch {
		case slices.Contains(missing, k):
			lines = append(lines, st.errorText.Render(truncate(markOff+" "+k+" faltando", width)))
		case defaults[k]:
			lines = append(lines, truncate(markOn+" "+k+" (padrão)", width))
		default:
			lines = append(lines, truncate(markOn+" "+k, width))
		}
	}
	if len(c.plan.Extra) > 0 {
		lines = append(lines, st.subtle.Render(truncate("fora do template: "+strings.Join(c.plan.Extra, ", "), width)))
	}
	return lines
}
