package presenter

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	targetModeShell     = "shell"
	targetModeClipboard = "clipboard"
	LoadModeCreated     = string(composeusecase.LoadCreated)
	LoadModeOverwritten = string(composeusecase.LoadOverwritten)
	LoadModeMerged      = string(composeusecase.LoadMerged)
	LoadModeExported    = "exported"
	LoadModeCopied      = "copied"
)

type planTarget struct {
	Path       string
	Exists     bool
	Gitignored composeusecase.GitignoreStatus
	Mode       string
}

type planFileTarget struct {
	Path       string                         `json:"path"`
	Exists     bool                           `json:"exists"`
	Gitignored composeusecase.GitignoreStatus `json:"gitignored"`
}

type planModeTarget struct {
	Mode string `json:"mode"`
}

func (t planTarget) MarshalJSON() ([]byte, error) {
	if t.Mode != "" {
		return json.Marshal(planModeTarget{Mode: t.Mode})
	}
	return json.Marshal(planFileTarget{Path: t.Path, Exists: t.Exists, Gitignored: t.Gitignored})
}

var (
	shellTarget     = planTarget{Mode: targetModeShell}
	clipboardTarget = planTarget{Mode: targetModeClipboard}
)

type planTemplate struct {
	Path  *string `json:"path"`
	Found bool    `json:"found"`
}

type planKey struct {
	Key     string   `json:"key"`
	From    *string  `json:"from"`
	Shadows []string `json:"shadows"`
	Default bool     `json:"default"`
}

type planEnvelope struct {
	SchemaVersion int          `json:"schema_version"`
	Envs          []string     `json:"envs"`
	Target        planTarget   `json:"target"`
	Template      planTemplate `json:"template"`
	Keys          []planKey    `json:"keys"`
	Conflicts     []string     `json:"conflicts"`
	Missing       []string     `json:"missing"`
	Extra         []string     `json:"extra"`
}

type loadEnvelope struct {
	planEnvelope
	Mode string `json:"mode"`
}

type SelectionEnvelope struct {
	SchemaVersion int        `json:"schema_version"`
	Envs          []string   `json:"envs"`
	Missing       []string   `json:"missing"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

type loadOutcome struct {
	plan    composeusecase.PlanView
	source  composeusecase.TemplateView
	target  planTarget
	mode    string
	out     string
	summary string
	asJSON  bool
}

func (p *Presenter) Plan(plan composeusecase.PlanView, source composeusecase.TemplateView, target composeusecase.TargetView, toFile bool) error {
	chosen := shellTarget
	if toFile {
		chosen = planTargetOf(target)
	}
	return writeJSON(p.stdout, planBuildEnvelope(plan, source, chosen))
}

func (p *Presenter) LoadedShell(result composeusecase.LoadShellExportsResult, source composeusecase.TemplateView, asJSON bool) error {
	return p.loadRun(loadOutcome{
		plan:    result.Plan,
		source:  source,
		target:  shellTarget,
		mode:    LoadModeExported,
		summary: fmt.Sprintf("Exportadas %d variáveis de %s no terminal.", result.Written, joined(result.Plan.Envs)),
		asJSON:  asJSON,
	})
}

func (p *Presenter) LoadedClipboard(result composeusecase.RenderEnvFileResult, source composeusecase.TemplateView, asJSON bool) error {
	return p.loadRun(loadOutcome{
		plan:    result.Plan,
		source:  source,
		target:  clipboardTarget,
		mode:    LoadModeCopied,
		summary: fmt.Sprintf("Copiado para o clipboard o .env com %d variáveis de %s.", len(result.Plan.Vars), joined(result.Plan.Envs)),
		asJSON:  asJSON,
	})
}

func (p *Presenter) LoadedFile(result composeusecase.LoadEnvFileResult, source composeusecase.TemplateView, out string, asJSON bool) error {
	mode := string(result.Mode)
	return p.loadRun(loadOutcome{
		plan:    result.Plan,
		source:  source,
		target:  planTargetOf(result.Target),
		mode:    mode,
		out:     out,
		summary: fmt.Sprintf("%s %s com %d variáveis de %s.", loadModeVerb(mode), out, result.Written, joined(result.Plan.Envs)),
		asJSON:  asJSON,
	})
}

func (p *Presenter) loadRun(outcome loadOutcome) error {
	if err := p.loadReport(outcome); err != nil {
		return err
	}
	if !outcome.asJSON {
		return nil
	}
	return writeJSON(p.stdout, loadEnvelope{planEnvelope: planBuildEnvelope(outcome.plan, outcome.source, outcome.target), Mode: outcome.mode})
}

func (p *Presenter) loadReport(outcome loadOutcome) error {
	var lines []string
	plan := outcome.plan
	if !outcome.asJSON {
		lines = append(lines, outcome.summary)
		if len(plan.Conflicts) > 0 {
			lines = append(lines, fmt.Sprintf("Conflitos resolvidos pela última env: %s.", joined(plan.Conflicts)))
		}
	}
	if len(plan.Missing) > 0 {
		lines = append(lines, fmt.Sprintf("aviso: chaves do template sem valor, não carregadas: %s", joined(plan.Missing)))
	}
	if outcome.target.Gitignored == composeusecase.GitignoreNotIgnored {
		lines = append(lines, fmt.Sprintf("aviso: %s não está coberto pelo .gitignore; adicione-o para não versionar segredos", outcome.out))
	}
	return p.infoLines(lines)
}

func loadModeVerb(mode string) string {
	switch mode {
	case LoadModeOverwritten:
		return "Substituído"
	case LoadModeMerged:
		return "Mesclado"
	default:
		return "Gravado"
	}
}

func planTargetOf(target composeusecase.TargetView) planTarget {
	return planTarget{Path: target.Path, Exists: target.Exists, Gitignored: target.Gitignored}
}

func planBuildEnvelope(plan composeusecase.PlanView, source composeusecase.TemplateView, target planTarget) planEnvelope {
	keys := make([]planKey, len(plan.Vars))
	for i, resolved := range plan.Vars {
		key := planKey{Key: resolved.Key, Shadows: resolved.Shadows, Default: resolved.Default}
		if resolved.From != "" {
			from := resolved.From
			key.From = &from
		}
		keys[i] = key
	}
	tmpl := planTemplate{Found: source.Found}
	if source.Path != "" {
		path := source.Path
		tmpl.Path = &path
	}
	return planEnvelope{
		SchemaVersion: SchemaVersion,
		Envs:          plan.Envs,
		Target:        target,
		Template:      tmpl,
		Keys:          keys,
		Conflicts:     plan.Conflicts,
		Missing:       plan.Missing,
		Extra:         plan.Extra,
	}
}

func (p *Presenter) Selection(result composeusecase.GetSelectionResult, asJSON bool) error {
	if asJSON {
		return writeJSON(p.stdout, selectionBuildEnvelope(result))
	}
	return p.selectionPrintHuman(result)
}

func selectionBuildEnvelope(result composeusecase.GetSelectionResult) SelectionEnvelope {
	envelope := SelectionEnvelope{
		SchemaVersion: SchemaVersion,
		Envs:          nonNil(result.Selection.Envs),
		Missing:       nonNil(result.Missing),
	}
	if result.Selection.Saved() {
		updatedAt := result.Selection.UpdatedAt
		envelope.UpdatedAt = &updatedAt
	}
	return envelope
}

func (p *Presenter) selectionPrintHuman(result composeusecase.GetSelectionResult) error {
	if len(result.Missing) > 0 {
		if err := p.Infof("fora do cofre, ignoradas: %s", joined(result.Missing)); err != nil {
			return err
		}
	}
	if len(result.Selection.Envs) == 0 {
		return p.Infof("nenhuma env selecionada")
	}
	var b strings.Builder
	for _, name := range result.Selection.Envs {
		fmt.Fprintf(&b, "%s\n", name)
	}
	return p.writeText(b.String())
}

func (p *Presenter) Script(script string) error {
	_, err := fmt.Fprint(p.stdout, script)
	return err
}

func (p *Presenter) ExecMissing(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return p.Infof("aviso: chaves do template sem valor: %s", joined(missing))
}

func ChildExited(result composeusecase.RunCommandResult) error {
	if result.ExitCode != ExitOK {
		return withExitCode(result.ExitCode, nil)
	}
	return nil
}

func ComposeError(err error) error {
	var duplicate *composeusecase.DuplicateEnvError
	var missing *composeusecase.EnvNotFoundError
	switch {
	case errors.As(err, &duplicate):
		return UsageError(err)
	case errors.As(err, &missing):
		return fmt.Errorf("%w: %q", vaultusecase.ErrEnvNotFound, missing.Name)
	}
	return err
}

func TemplateError(err error, path, onlyFlag, pathFlag string) error {
	switch {
	case errors.Is(err, composeusecase.ErrTemplateNotFound):
		return fmt.Errorf("%w: template %s não existe", ErrValidation, path)
	case errors.Is(err, composeusecase.ErrTemplateRequired):
		return fmt.Errorf("%w: --%s exige um template: passe --%s ou crie %s", ErrValidation, onlyFlag, pathFlag, composeusecase.DefaultTemplateFile)
	}
	return err
}

func FlagNeedsPath(name string) error {
	return UsageError(fmt.Errorf("--%s exige o caminho do arquivo", name))
}

func FlagsNeedFlag(first, second, needed string) error {
	return UsageError(fmt.Errorf("--%s e --%s só valem com --%s", first, second, needed))
}

func NoEnvGiven() error {
	return UsageError(errors.New("informe ao menos uma env com -e"))
}

func UnsupportedShell(dialect string, dialects []string) error {
	return UsageError(fmt.Errorf("shell %q não suportado, use %s", dialect, joined(dialects)))
}

func LoadNeedsWrapper(outFlag, dialect string) error {
	return UsageError(fmt.Errorf(
		"load sem --%s exporta no terminal atual e precisa do wrapper de shell: adicione %s ao rc do shell ou use --%s <arquivo>",
		outFlag, composeusecase.ShellInitLine(dialect), outFlag))
}

func LoadShellError(err error, outFlag, dialect string) error {
	switch {
	case errors.Is(err, composeusecase.ErrNoExportFile):
		return LoadNeedsWrapper(outFlag, dialect)
	case errors.Is(err, composeusecase.ErrTargetIsDirectory):
		return fmt.Errorf("%w: %s aponta para um diretório", ErrValidation, composeusecase.ExportFileVar)
	}
	return ComposeError(err)
}

func ClipboardError(err error) error {
	return fmt.Errorf("copiar para o clipboard: %w", err)
}

func LoadFileError(err error, out, forceFlag, mergeFlag string) error {
	switch {
	case errors.Is(err, composeusecase.ErrTargetExists):
		return fmt.Errorf("%w: %s (use --%s ou --%s)", ErrTargetExists, out, forceFlag, mergeFlag)
	case errors.Is(err, composeusecase.ErrTargetIsDirectory):
		return fmt.Errorf("%w: destino %s é um diretório", ErrValidation, out)
	}
	return ComposeError(err)
}
