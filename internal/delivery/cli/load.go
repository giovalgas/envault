package cli

import (
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

func newLoadCmd(app *App) *cobra.Command {
	opts := &loadOptions{}
	loadCmd := &cobra.Command{
		Use:   "load <env>...",
		Short: "Grava o .env combinando as envs na ordem dada (a última vence)",
		Long: "load combina as envs, aplica o template e grava o destino. Recusa quando o destino já existe, " +
			"a menos que --force (substitui) ou --merge (mantém chaves locais e atualiza as das envs) seja passado. " +
			"Nunca altera o .gitignore: só avisa quando o destino não está coberto por ele.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.force && opts.merge {
				return usageError(fmt.Errorf("--%s e --%s não podem ser usados juntos", loadFlagForce, loadFlagMerge))
			}
			source, err := opts.tmplFlags.resolve()
			if err != nil {
				return err
			}
			result, err := app.Compose().LoadEnvFile.Execute(cmd.Context(), composeusecase.LoadEnvFileInput{
				Envs:     args,
				Template: source.Template,
				Options:  opts.tmplFlags.options(),
				Target:   opts.out,
				Existing: opts.existing(),
			})
			if err != nil {
				return loadError(err, opts.out)
			}
			return loadRun(app, opts, result, source)
		},
	}
	loadCmd.Flags().StringVar(&opts.out, planFlagOut, planDefaultOut, "arquivo de destino")
	loadCmd.Flags().BoolVar(&opts.force, loadFlagForce, false, "substitui o destino se ele existir")
	loadCmd.Flags().BoolVar(&opts.merge, loadFlagMerge, false, "mescla com o destino existente, preservando a ordem e as chaves locais")
	opts.tmplFlags = templateBind(loadCmd)
	loadCmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "saída em JSON")
	return loadCmd
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

func loadError(err error, out string) error {
	switch {
	case errors.Is(err, composeusecase.ErrTargetExists):
		return fmt.Errorf("%w: %s (use --%s ou --%s)", ErrTargetExists, out, loadFlagForce, loadFlagMerge)
	case errors.Is(err, composeusecase.ErrTargetIsDirectory):
		return fmt.Errorf("%w: destino %s é um diretório", ErrValidation, out)
	}
	return composeError(err)
}

func loadRun(app *App, opts *loadOptions, result composeusecase.LoadEnvFileResult, source templateSource) error {
	target := planTargetOf(result.Target)
	mode := string(result.Mode)
	loadReport(app, opts, result.Plan, target, mode, result.Written)
	if !opts.asJSON {
		return nil
	}
	return writeJSON(app.Stdout, loadEnvelope{planEnvelope: planBuildEnvelope(result.Plan, source, target), Mode: mode})
}

func loadReport(app *App, opts *loadOptions, plan composedomain.Plan, target planTarget, mode string, written int) {
	if !opts.asJSON {
		app.Infof("%s %s com %d variáveis de %s.", loadModeVerb(mode), opts.out, written, strings.Join(plan.Envs, ", "))
		if len(plan.Conflicts) > 0 {
			app.Infof("Conflitos resolvidos pela última env: %s.", strings.Join(plan.Conflicts, ", "))
		}
	}
	if len(plan.Missing) > 0 {
		app.Infof("aviso: chaves do template sem valor, não gravadas: %s", strings.Join(plan.Missing, ", "))
	}
	if target.Gitignored == composedomain.GitignoreNotIgnored {
		app.Infof("aviso: %s não está coberto pelo .gitignore; adicione-o para não versionar segredos", opts.out)
	}
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
