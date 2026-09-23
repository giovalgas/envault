package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	composedomain "github.com/giovalgas/envault/internal/compose/domain"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
)

const (
	loadFlagForce       = "force"
	loadFlagMerge       = "merge"
	loadModeCreated     = string(composeusecase.LoadCreated)
	loadModeOverwritten = string(composeusecase.LoadOverwritten)
	loadModeMerged      = string(composeusecase.LoadMerged)
	loadModeExported    = "exported"
)

type loadEnvelope struct {
	planEnvelope
	Mode string `json:"mode"`
}

type loadOptions struct {
	out       string
	force     bool
	merge     bool
	asJSON    bool
	tmplFlags *templateFlags
}

type loadOutcome struct {
	plan    composedomain.Plan
	target  planTarget
	mode    string
	summary string
}

func newLoadCmd(app *App) *cobra.Command {
	opts := &loadOptions{}
	loadCmd := &cobra.Command{
		Use:   "load <env>...",
		Short: "Exporta no terminal atual as envs combinadas na ordem dada (a última vence), ou grava com --out",
		Long: "load combina as envs e aplica o template. Sem --out, exporta as variáveis no shell atual pelo wrapper " +
			"de shell-init, sem gravar .env e sem imprimir valores; sem o wrapper, sai com 2. Com --out, grava o arquivo " +
			"e recusa quando ele já existe, a menos que --force (substitui) ou --merge (mantém chaves locais e atualiza " +
			"as das envs) seja passado. Nunca altera o .gitignore: só avisa quando o arquivo não está coberto por ele.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toFile, err := outRequested(cmd, opts.out)
			if err != nil {
				return err
			}
			if err := opts.validate(toFile); err != nil {
				return err
			}
			if !toFile {
				return loadShell(cmd.Context(), app, opts, args)
			}
			return loadFile(cmd.Context(), app, opts, args)
		},
	}
	loadCmd.Flags().StringVar(&opts.out, planFlagOut, "", "arquivo de destino; sem ele, exporta no terminal pelo wrapper de shell-init")
	loadCmd.Flags().BoolVar(&opts.force, loadFlagForce, false, "substitui o arquivo de --out se ele existir")
	loadCmd.Flags().BoolVar(&opts.merge, loadFlagMerge, false, "mescla com o arquivo de --out existente, preservando a ordem e as chaves locais")
	opts.tmplFlags = templateBind(loadCmd)
	loadCmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "saída em JSON")
	return loadCmd
}

func (o *loadOptions) validate(toFile bool) error {
	if o.force && o.merge {
		return usageError(fmt.Errorf("--%s e --%s não podem ser usados juntos", loadFlagForce, loadFlagMerge))
	}
	if !toFile && (o.force || o.merge) {
		return usageError(fmt.Errorf("--%s e --%s só valem com --%s", loadFlagForce, loadFlagMerge, planFlagOut))
	}
	return nil
}

func (o *loadOptions) existing() composeusecase.ExistingTarget {
	switch {
	case o.force:
		return composeusecase.ReplaceExisting
	case o.merge:
		return composeusecase.MergeExisting
	default:
		return composeusecase.RefuseExisting
	}
}

func loadShell(ctx context.Context, app *App, opts *loadOptions, args []string) error {
	export := app.ShellExport()
	if !export.Active() {
		return loadWithoutWrapper(export.Dialect)
	}
	source, err := opts.tmplFlags.resolve()
	if err != nil {
		return err
	}
	result, err := app.Compose().LoadShellExports.Execute(ctx, composeusecase.LoadShellExportsInput{
		Envs:       args,
		Template:   source.Template,
		Options:    opts.tmplFlags.options(),
		Dialect:    export.Dialect,
		ExportFile: export.File,
	})
	switch {
	case errors.Is(err, composeusecase.ErrNoExportFile):
		return loadWithoutWrapper(export.Dialect)
	case errors.Is(err, composeusecase.ErrTargetIsDirectory):
		return fmt.Errorf("%w: %s aponta para um diretório", ErrValidation, composeusecase.ExportFileVar)
	case err != nil:
		return composeError(err)
	}
	return loadRun(app, opts, source, loadOutcome{
		plan:    result.Plan,
		target:  shellTarget,
		mode:    loadModeExported,
		summary: fmt.Sprintf("Exportadas %d variáveis de %s no terminal.", result.Written, strings.Join(result.Plan.Envs, ", ")),
	})
}

func loadWithoutWrapper(dialect string) error {
	return usageError(fmt.Errorf(
		"load sem --%s exporta no terminal atual e precisa do wrapper de shell: adicione %s ao rc do shell ou use --%s <arquivo>",
		planFlagOut, composeusecase.ShellInitLine(dialect), planFlagOut))
}

func loadFile(ctx context.Context, app *App, opts *loadOptions, args []string) error {
	source, err := opts.tmplFlags.resolve()
	if err != nil {
		return err
	}
	result, err := app.Compose().LoadEnvFile.Execute(ctx, composeusecase.LoadEnvFileInput{
		Envs:     args,
		Template: source.Template,
		Options:  opts.tmplFlags.options(),
		Target:   opts.out,
		Existing: opts.existing(),
	})
	if err != nil {
		return loadError(err, opts.out)
	}
	mode := string(result.Mode)
	return loadRun(app, opts, source, loadOutcome{
		plan:    result.Plan,
		target:  planTargetOf(result.Target),
		mode:    mode,
		summary: fmt.Sprintf("%s %s com %d variáveis de %s.", loadModeVerb(mode), opts.out, result.Written, strings.Join(result.Plan.Envs, ", ")),
	})
}

func loadError(err error, out string) error {
	switch {
	case errors.Is(err, composeusecase.ErrTargetExists):
		return fmt.Errorf("%w: %s (use --%s ou --%s)", ErrTargetExists, out, loadFlagForce, loadFlagMerge)
	case errors.Is(err, composeusecase.ErrTargetIsDirectory):
		return fmt.Errorf("%w: destino %s é um diretório", ErrValidation, out)
	}
	return composeError(err)
}

func loadRun(app *App, opts *loadOptions, source templateSource, outcome loadOutcome) error {
	if err := loadReport(app, opts, outcome); err != nil {
		return err
	}
	if !opts.asJSON {
		return nil
	}
	return writeJSON(app.Stdout, loadEnvelope{planEnvelope: planBuildEnvelope(outcome.plan, source, outcome.target), Mode: outcome.mode})
}

func loadReport(app *App, opts *loadOptions, outcome loadOutcome) error {
	var lines []string
	plan := outcome.plan
	if !opts.asJSON {
		lines = append(lines, outcome.summary)
		if len(plan.Conflicts) > 0 {
			lines = append(lines, fmt.Sprintf("Conflitos resolvidos pela última env: %s.", strings.Join(plan.Conflicts, ", ")))
		}
	}
	if len(plan.Missing) > 0 {
		lines = append(lines, fmt.Sprintf("aviso: chaves do template sem valor, não carregadas: %s", strings.Join(plan.Missing, ", ")))
	}
	if outcome.target.Gitignored == composedomain.GitignoreNotIgnored {
		lines = append(lines, fmt.Sprintf("aviso: %s não está coberto pelo .gitignore; adicione-o para não versionar segredos", opts.out))
	}
	for _, line := range lines {
		if err := app.Infof("%s", line); err != nil {
			return err
		}
	}
	return nil
}

func loadModeVerb(mode string) string {
	switch mode {
	case loadModeOverwritten:
		return "Substituído"
	case loadModeMerged:
		return "Mesclado"
	default:
		return "Gravado"
	}
}
