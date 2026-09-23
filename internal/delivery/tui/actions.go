package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	defaultEnvFile  = ".env"
	pathPromptLabel = "arquivo: "
)

type Deps struct {
	BeginCreateEnv *vaultusecase.BeginCreateEnv
	BeginEditEnv   *vaultusecase.BeginEditEnv
	ImportEnv      *vaultusecase.ImportEnv
	PlanLoad       *composeusecase.PlanLoad
	LoadEnvFile    *composeusecase.LoadEnvFile
	Dir            string
}

func Actions(deps Deps) map[Action]ActionHandler {
	actions := make(map[Action]ActionHandler, 4)
	if deps.BeginCreateEnv != nil {
		actions[ActionNew] = deps.newEnv
	}
	if deps.BeginEditEnv != nil {
		actions[ActionEdit] = deps.editEnv
	}
	if deps.ImportEnv != nil {
		actions[ActionImport] = deps.importEnv
	}
	if deps.PlanLoad != nil && deps.LoadEnvFile != nil {
		actions[ActionCompose] = deps.compose
	}
	return actions
}

type newEnvPromptMsg struct {
	begin *vaultusecase.BeginCreateEnv
}

type editorOpenMsg struct {
	draft *vaultusecase.EditDraft
}

type editorDoneMsg struct {
	draft  *vaultusecase.EditDraft
	result vault.EditResult
	err    error
}

type importPromptMsg struct {
	importer importer
}

type importNameMsg struct {
	importer importer
	path     string
}

type composeOpenMsg struct {
	names []string
	deps  composeDeps
}

type importer struct {
	uc  *vaultusecase.ImportEnv
	dir string
}

func (d Deps) newEnv(context.Context, ActionRequest) tea.Cmd {
	begin := d.BeginCreateEnv
	return func() tea.Msg { return newEnvPromptMsg{begin: begin} }
}

func (d Deps) editEnv(ctx context.Context, req ActionRequest) tea.Cmd {
	begin, name := d.BeginEditEnv, req.Focused.Name
	return func() tea.Msg {
		draft, err := begin.Execute(ctx, name)
		if err != nil {
			return ResultMsg{Err: fmt.Errorf("editar %q: %w", name, err)}
		}
		return editorOpenMsg{draft: draft}
	}
}

func (d Deps) importEnv(context.Context, ActionRequest) tea.Cmd {
	imp := importer{uc: d.ImportEnv, dir: d.Dir}
	return func() tea.Msg { return importPromptMsg{importer: imp} }
}

func (d Deps) compose(_ context.Context, req ActionRequest) tea.Cmd {
	names := make([]string, len(req.Marked))
	for i, env := range req.Marked {
		names[i] = env.Name
	}
	deps := composeDeps{plan: d.PlanLoad, load: d.LoadEnvFile, dir: d.Dir}
	return func() tea.Msg { return composeOpenMsg{names: names, deps: deps} }
}

func resolvePath(dir, path string) string {
	if dir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

func (m Model) openNewEnv(msg newEnvPromptMsg) Model {
	ctx, begin := m.ctx, msg.begin
	modal := newConfirm("Nova env", "Nome da nova env. O editor abre em seguida.").
		withInput(namePromptLabel, "").
		withChoices(
			choice{label: "criar", run: func(input string) (tea.Cmd, error) {
				name := strings.TrimSpace(input)
				if err := vault.ValidateName(name); err != nil {
					return nil, err
				}
				if m.list.hasEnv(name) {
					return nil, fmt.Errorf("%w: %q", vault.ErrEnvExists, name)
				}
				return func() tea.Msg {
					draft, err := begin.Execute(ctx, vault.Env{Name: name})
					if err != nil {
						return ResultMsg{Err: fmt.Errorf("criar %q: %w", name, err)}
					}
					return editorOpenMsg{draft: draft}
				}, nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

func (m Model) execEditor(draft *vaultusecase.EditDraft) tea.Cmd {
	return tea.Exec(draft.Process(m.ctx), func(runErr error) tea.Msg {
		result, err := draft.Review(runErr)
		return editorDoneMsg{draft: draft, result: result, err: err}
	})
}

func (m Model) onEditorOpen(msg editorOpenMsg) (tea.Model, tea.Cmd) {
	m.logger.Printf("editor aberto para %s", msg.draft.Name())
	if warnings := msg.draft.Warnings(); len(warnings) > 0 {
		m = m.setWarning(strings.Join(warnings, " "))
	}
	return m, m.execEditor(msg.draft)
}

func (m Model) onEditorDone(msg editorDoneMsg) (tea.Model, tea.Cmd) {
	draft, name := msg.draft, msg.draft.Name()
	switch {
	case errors.Is(msg.err, vault.ErrEditReopen):
		m.logger.Printf("conteúdo inválido em %s, reabrindo o editor", name)
		return m, m.execEditor(draft)
	case errors.Is(msg.err, vault.ErrEditCanceled):
		return m.setStatus(editCanceledText(draft), nil), nil
	case msg.err != nil:
		return m.setStatus("", fmt.Errorf("editor de %q: %w", name, msg.err)), nil
	case !msg.result.Changed:
		return m.setStatus(fmt.Sprintf("nada mudou em %s", name), nil), nil
	}
	return m.openDiff(draft, msg.result), nil
}

func editCanceledText(draft *vaultusecase.EditDraft) string {
	if draft.IsNew() {
		return fmt.Sprintf("criação de %s cancelada", draft.Name())
	}
	return fmt.Sprintf("edição de %s cancelada", draft.Name())
}

func (m Model) openDiff(draft *vaultusecase.EditDraft, result vault.EditResult) Model {
	ctx, name := m.ctx, draft.Name()
	title, verb, done := "Gravar alterações em "+name, "gravar", "atualizada"
	if draft.IsNew() {
		title, verb, done = "Criar "+name, "criar", "criada"
	}
	body := append([]string{"Mudanças por chave, sem valores:"}, result.Diff.Lines()...)
	modal := newConfirm(title, body...).withChoices(
		choice{label: verb, run: func(string) (tea.Cmd, error) {
			return func() tea.Msg {
				env, err := draft.Apply(ctx, result)
				if err != nil {
					return ResultMsg{Err: fmt.Errorf("gravar %q: %w", name, err)}
				}
				return ResultMsg{Status: fmt.Sprintf("%s %s com %s", name, done, keyCount(len(env.Vars))), Focus: env.Name}
			}, nil
		}},
		choice{label: "descartar", run: func(string) (tea.Cmd, error) {
			return statusCmd(fmt.Sprintf("alterações em %s descartadas", name)), nil
		}},
	)
	m.modal = &modal
	return m
}

func statusCmd(text string) tea.Cmd {
	return func() tea.Msg { return statusMsg{text: text} }
}

func (m Model) openImportPath(msg importPromptMsg) Model {
	imp := msg.importer
	modal := newConfirm("Importar .env", "Arquivo .env a importar:").
		withInput(pathPromptLabel, defaultEnvFile).
		withChoices(
			choice{label: "continuar", run: func(input string) (tea.Cmd, error) {
				path := strings.TrimSpace(input)
				if path == "" {
					return nil, errors.New("informe o arquivo a importar")
				}
				return func() tea.Msg { return importNameMsg{importer: imp, path: path} }, nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

func (m Model) openImportName(msg importNameMsg) Model {
	imp, path := msg.importer, msg.path
	modal := newConfirm("Importar "+path, "Nome da env que recebe as variáveis:").
		withInput(namePromptLabel, "").
		withChoices(
			choice{label: "importar", run: func(input string) (tea.Cmd, error) {
				name := strings.TrimSpace(input)
				if err := vault.ValidateName(name); err != nil {
					return nil, err
				}
				if m.list.hasEnv(name) {
					return func() tea.Msg { return importReplaceMsg{importer: imp, path: path, name: name} }, nil
				}
				return m.importCmd(imp, path, name), nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

type importReplaceMsg struct {
	importer importer
	path     string
	name     string
}

func (m Model) openImportReplace(msg importReplaceMsg) Model {
	modal := newConfirm("Substituir "+msg.name,
		fmt.Sprintf("%s já existe e passará a ter exatamente as variáveis de %s.", msg.name, msg.path)).
		withChoices(
			choice{label: "substituir", run: func(string) (tea.Cmd, error) {
				return m.importCmd(msg.importer, msg.path, msg.name), nil
			}},
			cancelChoice(),
		)
	m.modal = &modal
	return m
}

func (m Model) importCmd(imp importer, path, name string) tea.Cmd {
	ctx, resolved := m.ctx, resolvePath(imp.dir, path)
	source := vaultusecase.EnvSource{Name: path, Read: func() ([]byte, error) {
		data, err := os.ReadFile(filepath.Clean(resolved))
		if err != nil {
			return nil, fmt.Errorf("ler %s: %w", path, err)
		}
		return data, nil
	}}
	return func() tea.Msg {
		result, err := imp.uc.Execute(ctx, vaultusecase.ImportEnvInput{Name: name, From: source})
		if err != nil {
			return ResultMsg{Err: fmt.Errorf("importar %s: %w", path, err)}
		}
		verb := "criada"
		if result.Replaced {
			verb = "substituída"
		}
		return ResultMsg{Status: fmt.Sprintf("%s %s com %s de %s", name, verb, keyCount(len(result.Env.Vars)), path), Focus: name}
	}
}
