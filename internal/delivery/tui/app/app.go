package app

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

	"github.com/giovalgas/envault/internal/delivery/tui/screen/compose"
	"github.com/giovalgas/envault/internal/delivery/tui/screen/confirm"
	"github.com/giovalgas/envault/internal/delivery/tui/screen/detail"
	"github.com/giovalgas/envault/internal/delivery/tui/screen/list"
	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	defaultWidth    = 80
	defaultHeight   = 24
	chromeLines     = 3
	copySuffix      = "-copy"
	debugLogName    = "envault-debug.log"
	debugLogPrefix  = "envault"
	namePromptLabel = "nome: "
)

type Store interface {
	List(ctx context.Context) ([]vaultusecase.EnvView, error)
	Get(ctx context.Context, name string) (vaultusecase.EnvView, error)
	Copy(ctx context.Context, src, dst string) (vaultusecase.EnvView, error)
	Rename(ctx context.Context, oldName, newName string) (vaultusecase.EnvView, error)
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
	Focused *vaultusecase.EnvView
	Marked  []vaultusecase.EnvView
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
	Selection SelectionStore
}

type screen int

const (
	screenList screen = iota
	screenDetail
	screenCompose
)

type envsLoadedMsg struct {
	envs    []vaultusecase.EnvView
	err     error
	focus   string
	status  string
	warning string
	restore *restoredSelection
}

type detailLoadedMsg struct {
	env vaultusecase.EnvView
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
	keys       theme.KeyMap
	styles     theme.Styles
	help       help.Model
	width      int
	height     int
	screen     screen
	list       list.Model
	detail     detail.Model
	compose    compose.Model
	modal      *confirm.Model
	showHelp   bool
	status     string
	statusErr  bool
	statusWarn bool
	initVault  InitVaultFunc
	exported   string
	selection  *selectionSaver
	saveSeq    int
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
		keys:      theme.DefaultKeyMap(),
		styles:    theme.DefaultStyles(),
		help:      help.New(),
		width:     defaultWidth,
		height:    defaultHeight,
		list:      list.New(),
		initVault: opts.InitVault,
		selection: newSelectionSaver(opts.Selection),
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
	if ok {
		m.selection.wait()
	}
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
		return m.firstLoadCmd("")
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
		result := ResultMsg{Status: fmt.Sprintf("%s renomeada para %s", oldName, newName), Focus: newName}
		return envRenamedMsg{oldName: oldName, newName: newName, result: result}
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
		return m.onEnvsLoaded(msg)
	case envRenamedMsg:
		return m.onEnvRenamed(msg)
	case selectionSavedMsg:
		return m.onSelectionSaved(msg), nil
	case detailLoadedMsg:
		return m.onDetailLoaded(msg), nil
	case detail.CopiedMsg:
		return m.setStatus(msg.Status, msg.Err), nil
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
	case compose.PlanMsg:
		return m.onComposePlan(msg), nil
	case compose.WrittenMsg:
		return m.onComposeWritten(msg)
	case compose.ExportedMsg:
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
	return m, m.firstLoadCmd(status)
}

func (m Model) onEnvsLoaded(msg envsLoadedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.logger.Printf("carregar envs: %v", msg.err)
		m.list = m.list.MarkFailed()
		return m.setStatus("", fmt.Errorf("carregar cofre: %w", msg.err)), nil
	}
	m.logger.Printf("envs carregadas: %d", len(msg.envs))
	before := m.list.Marked()
	m.list = m.list.SetEnvs(msg.envs, msg.focus)
	if m.screen == screenDetail {
		m = m.refreshDetail(msg.envs)
	}
	switch {
	case msg.warning != "":
		m = m.setWarning(joinStatus(msg.status, msg.warning))
	case msg.status != "":
		m = m.setStatus(msg.status, nil)
	}
	m, before = m.restoreSelection(msg, before)
	return m.persistSelection(before)
}

func joinStatus(parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "; ")
}

func (m Model) refreshDetail(envs []vaultusecase.EnvView) Model {
	refreshed, ok := m.detail.Refresh(envs)
	m.detail = refreshed
	if !ok {
		m.screen = screenList
	}
	return m
}

func (m Model) onDetailLoaded(msg detailLoadedMsg) Model {
	if msg.err != nil {
		m.logger.Printf("abrir detalhe: %v", msg.err)
		return m.setStatus("", fmt.Errorf("abrir env: %w", msg.err))
	}
	m.detail = detail.New(msg.env, m.clipboard)
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
		modal, cmd, closed := m.modal.Update(msg)
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
	before := m.list.Marked()
	updated, cmd, intent := m.list.Update(msg)
	m.list = updated
	switch intent {
	case list.IntentQuit:
		return m, m.quitCmd()
	case list.IntentHelp:
		m.showHelp = true
	case list.IntentOpen:
		if env, ok := m.list.Focused(); ok {
			return m, m.getCmd(env.Name)
		}
	case list.IntentDuplicate:
		return m.openDuplicate(), nil
	case list.IntentRename:
		return m.openRename(), nil
	case list.IntentDelete:
		return m.openDelete(), nil
	case list.IntentNew:
		return m.runAction(ActionNew)
	case list.IntentEdit:
		return m.runAction(ActionEdit)
	case list.IntentImport:
		return m.runAction(ActionImport)
	case list.IntentCompose:
		return m.runAction(ActionCompose)
	}
	m, save := m.persistSelection(before)
	return m, tea.Batch(cmd, save)
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, cmd, intent := m.detail.Update(msg)
	m.detail = updated
	switch intent {
	case detail.IntentBack:
		m.screen = screenList
		m.detail = detail.Model{}
	case detail.IntentHelp:
		m.showHelp = true
	}
	return m, cmd
}

func (m Model) runAction(action Action) (tea.Model, tea.Cmd) {
	handler, ok := m.actions[action]
	if !ok || handler == nil {
		return m.setStatus("", fmt.Errorf("ação %q ainda não disponível", action)), nil
	}
	req := ActionRequest{Marked: m.list.MarkedEnvs()}
	if env, found := m.list.Focused(); found {
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
	env, ok := m.list.Focused()
	if !ok {
		return m
	}
	src := env.Name
	modal := confirm.New("Duplicar "+src, "Nome da nova env:").
		WithInput(namePromptLabel, src+copySuffix).
		WithChoices(
			confirm.Choice{Label: "duplicar", Run: func(input string) (tea.Cmd, error) {
				dst := strings.TrimSpace(input)
				if err := vaultusecase.ValidateName(dst); err != nil {
					return nil, err
				}
				return m.copyEnvCmd(src, dst), nil
			}},
			confirm.Cancel(),
		)
	m.modal = &modal
	return m
}

func (m Model) openRename() Model {
	env, ok := m.list.Focused()
	if !ok {
		return m
	}
	oldName := env.Name
	modal := confirm.New("Renomear "+oldName, "Novo nome da env:").
		WithInput(namePromptLabel, oldName).
		WithChoices(
			confirm.Choice{Label: "renomear", Run: func(input string) (tea.Cmd, error) {
				newName := strings.TrimSpace(input)
				if newName == oldName {
					return nil, nil
				}
				if err := vaultusecase.ValidateName(newName); err != nil {
					return nil, err
				}
				return m.renameEnvCmd(oldName, newName), nil
			}},
			confirm.Cancel(),
		)
	m.modal = &modal
	return m
}

func (m Model) openDelete() Model {
	env, ok := m.list.Focused()
	if !ok {
		return m
	}
	name := env.Name
	modal := confirm.New("Apagar "+name,
		"Esta ação não pode ser desfeita.",
		fmt.Sprintf("Digite %s para confirmar.", name)).
		WithInput(namePromptLabel, "").
		WithChoices(
			confirm.Choice{Label: "apagar", Run: func(input string) (tea.Cmd, error) {
				if input != name {
					return nil, errNameMismatch
				}
				return m.deleteEnvCmd(name), nil
			}},
			confirm.Cancel(),
		)
	m.modal = &modal
	return m
}

func (m Model) View() string {
	chrome := chromeLines
	if m.showsSelection() {
		chrome++
	}
	bodyHeight := max(m.height-chrome, 3)
	var body string
	switch {
	case m.showHelp:
		body = theme.FullHelp(m.styles, m.keys, m.width, bodyHeight)
	case m.screen == screenDetail:
		body = m.detail.View(m.styles, m.width, bodyHeight)
	case m.screen == screenCompose:
		body = m.compose.View(m.styles, m.width, bodyHeight)
	default:
		body = m.list.View(m.styles, m.width, bodyHeight)
	}
	if m.modal != nil {
		body = lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, m.modal.View(m.styles, m.width))
	}
	parts := []string{m.headerView()}
	if m.showsSelection() {
		parts = append(parts, m.list.SelectionView(m.styles, m.width))
	}
	parts = append(parts, body, m.statusView(), m.help.View(m.activeHelp()))
	return strings.Join(parts, "\n")
}

func (m Model) headerView() string {
	return m.styles.Title.Render("envault")
}

func (m Model) statusView() string {
	if m.status == "" {
		return ""
	}
	text := theme.Truncate(m.status, m.width)
	switch {
	case m.statusErr:
		return m.styles.ErrorText.Render(text)
	case m.statusWarn:
		return m.styles.WarnText.Render(text)
	}
	return m.styles.Status.Render(text)
}

func (m Model) activeHelp() help.KeyMap {
	switch {
	case m.modal != nil:
		return m.keys.ModalHelp()
	case m.showHelp:
		return m.keys.HelpScreenHelp()
	case m.screen == screenDetail:
		return m.keys.DetailHelp()
	case m.screen == screenCompose:
		return m.keys.ComposeHelp()
	case m.list.Filtering():
		return m.keys.FilterHelp()
	default:
		return m.keys.ListHelp()
	}
}
