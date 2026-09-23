package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

const (
	defaultWidth    = 80
	defaultHeight   = 24
	chromeLines     = 3
	copySuffix      = "-copy"
	debugLogName    = "envault-debug.log"
	debugLogPrefix  = "envault"
	helpKeyColumn   = 12
	namePromptLabel = "nome: "
)

type Store interface {
	List(ctx context.Context) ([]vault.Env, error)
	Get(ctx context.Context, name string) (vault.Env, error)
	Copy(ctx context.Context, src, dst string) (vault.Env, error)
	Rename(ctx context.Context, oldName, newName string) (vault.Env, error)
	Delete(ctx context.Context, name string) error
}

type Action int

const (
	ActionNew Action = iota + 1
	ActionEdit
	ActionImport
	ActionCompose
)

func (a Action) String() string {
	switch a {
	case ActionNew:
		return "new"
	case ActionEdit:
		return "edit"
	case ActionImport:
		return "import"
	case ActionCompose:
		return "compose"
	default:
		return fmt.Sprintf("action(%d)", int(a))
	}
}

type ActionRequest struct {
	Focused *vault.Env
	Marked  []vault.Env
}

type ActionHandler func(ctx context.Context, req ActionRequest) tea.Cmd

type ResultMsg struct {
	Status  string
	Warning string
	Focus   string
	Err     error
}

type InitVaultFunc func(ctx context.Context) (created bool, location string, err error)

type Options struct {
	Clipboard func(string) error
	Actions   map[Action]ActionHandler
	Logger    *log.Logger
	Debug     bool
	LogPath   string
	Input     io.Reader
	Output    io.Writer
	Report    io.Writer
	InitVault InitVaultFunc
}

type screen int

const (
	screenList screen = iota
	screenDetail
	screenCompose
)

type envsLoadedMsg struct {
	envs    []vault.Env
	err     error
	focus   string
	status  string
	warning string
}

type detailLoadedMsg struct {
	env vault.Env
	err error
}

type statusMsg struct {
	text string
	err  error
}

type vaultInitializedMsg struct {
	created  bool
	location string
	err      error
}

var errNameMismatch = errors.New("o nome digitado não confere")

type Model struct {
	ctx        context.Context
	store      Store
	clipboard  func(string) error
	actions    map[Action]ActionHandler
	logger     *log.Logger
	keys       keyMap
	styles     styles
	help       help.Model
	width      int
	height     int
	screen     screen
	list       listModel
	detail     detailModel
	compose    composeModel
	modal      *confirmModel
	showHelp   bool
	status     string
	statusErr  bool
	statusWarn bool
	initVault  InitVaultFunc
	exported   string
}

func New(ctx context.Context, store Store, opts Options) Model {
	write := opts.Clipboard
	if write == nil {
		write = clipboard.WriteAll
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	actions := make(map[Action]ActionHandler, len(opts.Actions))
	for action, handler := range opts.Actions {
		actions[action] = handler
	}
	return Model{
		ctx:       ctx,
		store:     store,
		clipboard: write,
		actions:   actions,
		logger:    logger,
		keys:      defaultKeyMap(),
		styles:    defaultStyles(),
		help:      help.New(),
		width:     defaultWidth,
		height:    defaultHeight,
		list:      newListModel(),
		initVault: opts.InitVault,
	}
}

func Run(ctx context.Context, store Store, opts Options) (err error) {
	if opts.Debug && opts.Logger == nil {
		path := opts.LogPath
		if path == "" {
			path = filepath.Join(os.TempDir(), debugLogName)
		}
		logger := log.New(io.Discard, "", log.LstdFlags)
		f, openErr := tea.LogToFileWith(path, debugLogPrefix, logger)
		if openErr != nil {
			return fmt.Errorf("abrir log de depuração: %w", openErr)
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("fechar log de depuração: %w", closeErr)
			}
		}()
		opts.Logger = logger
	}
	programOpts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithContext(ctx)}
	if opts.Input != nil {
		programOpts = append(programOpts, tea.WithInput(opts.Input))
	}
	if opts.Output != nil {
		programOpts = append(programOpts, tea.WithOutput(opts.Output))
	}
	final, err := tea.NewProgram(New(ctx, store, opts), programOpts...).Run()
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("executar TUI: %w", err)
	}
	return reportExported(opts.Report, final)
}

func reportExported(w io.Writer, final tea.Model) error {
	m, ok := final.(Model)
	if !ok || m.exported == "" || w == nil {
		return nil
	}
	if _, err := fmt.Fprintf(w, "envault: %s\n", m.exported); err != nil {
		return fmt.Errorf("informar exportação: %w", err)
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return m.initCmd()
}

func (m Model) initCmd() tea.Cmd {
	initVault := m.initVault
	if initVault == nil {
		return m.loadCmd("", "")
	}
	ctx := m.ctx
	return func() tea.Msg {
		created, location, err := initVault(ctx)
		return vaultInitializedMsg{created: created, location: location, err: err}
	}
}

func (m Model) loadCmd(focus, status string) tea.Cmd {
	return m.reloadCmd(ResultMsg{Focus: focus, Status: status})
}

func (m Model) reloadCmd(result ResultMsg) tea.Cmd {
	ctx, store := m.ctx, m.store
	return func() tea.Msg {
		envs, err := store.List(ctx)
		return envsLoadedMsg{envs: envs, err: err, focus: result.Focus, status: result.Status, warning: result.Warning}
	}
}

func (m Model) getCmd(name string) tea.Cmd {
	ctx, store := m.ctx, m.store
	return func() tea.Msg {
		env, err := store.Get(ctx, name)
		return detailLoadedMsg{env: env, err: err}
	}
}

func (m Model) copyEnvCmd(src, dst string) tea.Cmd {
	ctx, store := m.ctx, m.store
	return func() tea.Msg {
		if _, err := store.Copy(ctx, src, dst); err != nil {
			return ResultMsg{Err: fmt.Errorf("duplicar %q: %w", src, err)}
		}
		return ResultMsg{Status: fmt.Sprintf("%s duplicada como %s", src, dst), Focus: dst}
	}
}

func (m Model) renameEnvCmd(oldName, newName string) tea.Cmd {
	ctx, store := m.ctx, m.store
	return func() tea.Msg {
		if _, err := store.Rename(ctx, oldName, newName); err != nil {
			return ResultMsg{Err: fmt.Errorf("renomear %q: %w", oldName, err)}
		}
		return ResultMsg{Status: fmt.Sprintf("%s renomeada para %s", oldName, newName), Focus: newName}
	}
}

func (m Model) deleteEnvCmd(name string) tea.Cmd {
	ctx, store := m.ctx, m.store
	return func() tea.Msg {
		if err := store.Delete(ctx, name); err != nil {
			return ResultMsg{Err: fmt.Errorf("apagar %q: %w", name, err)}
		}
		return ResultMsg{Status: fmt.Sprintf("%s apagada", name)}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		return m, nil
	case vaultInitializedMsg:
		return m.onVaultInitialized(msg)
	case envsLoadedMsg:
		return m.onEnvsLoaded(msg), nil
	case detailLoadedMsg:
		return m.onDetailLoaded(msg), nil
	case ResultMsg:
		return m.onResult(msg)
	case statusMsg:
		return m.setStatus(msg.text, msg.err), nil
	case newEnvPromptMsg:
		return m.openNewEnv(msg), nil
	case editorOpenMsg:
		return m.onEditorOpen(msg)
	case editorDoneMsg:
		return m.onEditorDone(msg)
	case importPromptMsg:
		return m.openImportPath(msg), nil
	case importNameMsg:
		return m.openImportName(msg), nil
	case importReplaceMsg:
		return m.openImportReplace(msg), nil
	case composeOpenMsg:
		return m.openCompose(msg)
	case composePlanMsg:
		return m.onComposePlan(msg), nil
	case composeWrittenMsg:
		return m.onComposeWritten(msg)
	case composeExportedMsg:
		return m.onComposeExported(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) onVaultInitialized(msg vaultInitializedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.logger.Printf("inicializar cofre: %v", msg.err)
		return m.setStatus("", fmt.Errorf("inicializar cofre: %w", msg.err)), nil
	}
	status := ""
	if msg.created {
		status = fmt.Sprintf("cofre criado em %s", msg.location)
	}
	return m, m.loadCmd("", status)
}

func (m Model) onEnvsLoaded(msg envsLoadedMsg) Model {
	if msg.err != nil {
		m.logger.Printf("carregar envs: %v", msg.err)
		m.list = m.list.markFailed()
		return m.setStatus("", fmt.Errorf("carregar cofre: %w", msg.err))
	}
	m.logger.Printf("envs carregadas: %d", len(msg.envs))
	m.list = m.list.setEnvs(msg.envs, msg.focus)
	if m.screen == screenDetail {
		m = m.refreshDetail(msg.envs)
	}
	switch {
	case msg.warning != "":
		m = m.setWarning(joinStatus(msg.status, msg.warning))
	case msg.status != "":
		m = m.setStatus(msg.status, nil)
	}
	return m
}

func joinStatus(parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "; ")
}

func (m Model) refreshDetail(envs []vault.Env) Model {
	for _, env := range envs {
		if env.Name == m.detail.env.Name {
			cursor := min(m.detail.cursor, max(len(env.Vars)-1, 0))
			m.detail = newDetailModel(env)
			m.detail.cursor = cursor
			return m
		}
	}
	m.screen = screenList
	m.detail = detailModel{}
	return m
}

func (m Model) onDetailLoaded(msg detailLoadedMsg) Model {
	if msg.err != nil {
		m.logger.Printf("abrir detalhe: %v", msg.err)
		return m.setStatus("", fmt.Errorf("abrir env: %w", msg.err))
	}
	m.detail = newDetailModel(msg.env)
	m.screen = screenDetail
	return m.setStatus("", nil)
}

func (m Model) onResult(msg ResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.logger.Printf("ação falhou: %v", msg.Err)
		return m.setStatus("", msg.Err), nil
	}
	return m, m.reloadCmd(msg)
}

func (m Model) setStatus(text string, err error) Model {
	m.statusWarn = false
	if err != nil {
		m.status = err.Error()
		m.statusErr = true
		return m
	}
	m.status = text
	m.statusErr = false
	return m
}

func (m Model) setWarning(text string) Model {
	m = m.setStatus(text, nil)
	m.statusWarn = true
	return m
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal != nil {
		modal, cmd, closed := m.modal.update(msg, m.keys)
		if closed {
			m.modal = nil
			return m, cmd
		}
		m.modal = &modal
		return m, cmd
	}
	if m.showHelp {
		if key.Matches(msg, m.keys.Help, m.keys.Back, m.keys.Quit) {
			m.showHelp = false
		}
		return m, nil
	}
	switch m.screen {
	case screenDetail:
		return m.updateDetail(msg)
	case screenCompose:
		return m.updateCompose(msg)
	}
	return m.updateList(msg)
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.list.filtering {
		switch {
		case key.Matches(msg, m.keys.Back):
			m.list = m.list.clearFilter()
			return m, nil
		case key.Matches(msg, m.keys.Confirm):
			m.list = m.list.stopFilter()
			return m, nil
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.updateFilter(msg)
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		if m.list.hasFilter() {
			m.list = m.list.clearFilter()
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.list = m.list.move(-1)
	case key.Matches(msg, m.keys.Down):
		m.list = m.list.move(1)
	case key.Matches(msg, m.keys.Mark):
		m.list = m.list.toggleMark()
	case key.Matches(msg, m.keys.Filter):
		m.list = m.list.startFilter()
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
	case key.Matches(msg, m.keys.Open):
		if env, ok := m.list.focused(); ok {
			return m, m.getCmd(env.Name)
		}
	case key.Matches(msg, m.keys.Duplicate):
		return m.openDuplicate(), nil
	case key.Matches(msg, m.keys.Rename):
		return m.openRename(), nil
	case key.Matches(msg, m.keys.Delete):
		return m.openDelete(), nil
	case key.Matches(msg, m.keys.New):
		return m.runAction(ActionNew)
	case key.Matches(msg, m.keys.Edit):
		return m.runAction(ActionEdit)
	case key.Matches(msg, m.keys.Import):
		return m.runAction(ActionImport)
	case key.Matches(msg, m.keys.Compose):
		return m.runAction(ActionCompose)
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit, m.keys.Back):
		m.screen = screenList
		m.detail = detailModel{}
	case key.Matches(msg, m.keys.Up):
		m.detail = m.detail.move(-1)
	case key.Matches(msg, m.keys.Down):
		m.detail = m.detail.move(1)
	case key.Matches(msg, m.keys.Reveal):
		m.detail = m.detail.toggleReveal()
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
	case key.Matches(msg, m.keys.Copy):
		if v, ok := m.detail.focusedVar(); ok {
			return m, copyValueCmd(m.clipboard, v)
		}
	}
	return m, nil
}

func (m Model) runAction(action Action) (tea.Model, tea.Cmd) {
	handler, ok := m.actions[action]
	if !ok || handler == nil {
		return m.setStatus("", fmt.Errorf("ação %q ainda não disponível", action)), nil
	}
	req := ActionRequest{Marked: m.list.markedEnvs()}
	if env, found := m.list.focused(); found {
		focused := env.Clone()
		req.Focused = &focused
	}
	switch {
	case action == ActionEdit && req.Focused == nil:
		return m.setStatus("", errors.New("nenhuma env selecionada para editar")), nil
	case action == ActionCompose && len(req.Marked) == 0:
		return m.setStatus("", errors.New("marque ao menos uma env com space antes de carregar")), nil
	}
	m.logger.Printf("ação %s", action)
	return m, handler(m.ctx, req)
}

func (m Model) openDuplicate() Model {
	env, ok := m.list.focused()
	if !ok {
		return m
	}
	src := env.Name
	modal := newConfirm("Duplicar "+src, "Nome da nova env:").
		withInput(namePromptLabel, src+copySuffix).
		withChoices(
			choice{label: "duplicar", run: func(input string) (tea.Cmd, error) {
				dst := strings.TrimSpace(input)
				if err := vault.ValidateName(dst); err != nil {
					return nil, err
				}
				return m.copyEnvCmd(src, dst), nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

func (m Model) openRename() Model {
	env, ok := m.list.focused()
	if !ok {
		return m
	}
	oldName := env.Name
	modal := newConfirm("Renomear "+oldName, "Novo nome da env:").
		withInput(namePromptLabel, oldName).
		withChoices(
			choice{label: "renomear", run: func(input string) (tea.Cmd, error) {
				newName := strings.TrimSpace(input)
				if newName == oldName {
					return nil, nil
				}
				if err := vault.ValidateName(newName); err != nil {
					return nil, err
				}
				return m.renameEnvCmd(oldName, newName), nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

func (m Model) openDelete() Model {
	env, ok := m.list.focused()
	if !ok {
		return m
	}
	name := env.Name
	modal := newConfirm("Apagar "+name,
		"Esta ação não pode ser desfeita.",
		fmt.Sprintf("Digite %s para confirmar.", name)).
		withInput(namePromptLabel, "").
		withChoices(
			choice{label: "apagar", run: func(input string) (tea.Cmd, error) {
				if input != name {
					return nil, errNameMismatch
				}
				return m.deleteEnvCmd(name), nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

func (m Model) View() string {
	bodyHeight := max(m.height-chromeLines, 3)
	var body string
	switch {
	case m.showHelp:
		body = m.fullHelpView(bodyHeight)
	case m.screen == screenDetail:
		body = m.detail.view(m.styles, m.width, bodyHeight)
	case m.screen == screenCompose:
		body = m.compose.view(m.styles, m.width, bodyHeight)
	default:
		body = m.list.view(m.styles, m.width, bodyHeight)
	}
	if m.modal != nil {
		body = lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, m.modal.view(m.styles, m.width))
	}
	return strings.Join([]string{m.headerView(), body, m.statusView(), m.help.View(m.activeHelp())}, "\n")
}

func (m Model) headerView() string {
	title := m.styles.title.Render("envault")
	info := fmt.Sprintf("%d envs", len(m.list.envs))
	switch n := len(m.list.marked); n {
	case 0:
	case 1:
		info += ", 1 marcada"
	default:
		info += fmt.Sprintf(", %d marcadas", n)
	}
	return title + "  " + m.styles.subtle.Render(info)
}

func (m Model) statusView() string {
	if m.status == "" {
		return ""
	}
	text := truncate(m.status, m.width)
	switch {
	case m.statusErr:
		return m.styles.errorText.Render(text)
	case m.statusWarn:
		return m.styles.warnText.Render(text)
	}
	return m.styles.status.Render(text)
}

func (m Model) activeHelp() help.KeyMap {
	switch {
	case m.modal != nil:
		return m.keys.modalHelp()
	case m.showHelp:
		return m.keys.helpScreenHelp()
	case m.screen == screenDetail:
		return m.keys.detailHelp()
	case m.screen == screenCompose:
		return m.keys.composeHelp()
	case m.list.filtering:
		return m.keys.filterHelp()
	default:
		return m.keys.listHelp()
	}
}

func (m Model) fullHelpView(height int) string {
	groups := m.keys.allGroups()
	split := (len(groups) + 1) / 2
	left := m.helpColumn(groups[:split])
	right := m.helpColumn(groups[split:])
	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)
	content := m.styles.title.Render("Atalhos") + "\n\n" + columns
	return m.styles.pane.Width(max(m.width-paneBorder, 1)).Height(max(height-paneBorder, 1)).Render(content)
}

func (m Model) helpColumn(groups [][]key.Binding) string {
	var lines []string
	for i, group := range groups {
		if i > 0 {
			lines = append(lines, "")
		}
		for _, b := range group {
			h := b.Help()
			lines = append(lines, m.styles.helpKey.Render(padRight(h.Key, helpKeyColumn))+m.styles.helpDesc.Render(h.Desc))
		}
	}
	return strings.Join(lines, "\n")
}
