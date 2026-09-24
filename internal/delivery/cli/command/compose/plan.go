package compose

import (
	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

const planFlagOut = "out"

func outRequested(cmd *cobra.Command, out string) (bool, error) {
	if !cmd.Flags().Changed(planFlagOut) {
		return false, nil
	}
	if out == "" {
		return false, presenter.FlagNeedsPath(planFlagOut)
	}
	return true, nil
}

func NewPlanCmd(a *app.App) *cobra.Command {
	var out string
	var tmplFlags *templateFlags
	planCmd := &cobra.Command{
		Use:   "plan <env>...",
		Short: "Mostra em JSON o que load faria, sem gravar nada e sem valores",
		Long: "plan combina as envs na ordem dada (a última vence), aplica o template e descreve o resultado em JSON: " +
			"chaves, origem, conflitos, faltando e extras. Com --out, target descreve o arquivo (path, exists, gitignored); " +
			"sem --out, target é {\"mode\":\"shell\"}, o destino do load no terminal. Nunca grava arquivo e nunca inclui valores.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toFile, err := outRequested(cmd, out)
			if err != nil {
				return err
			}
			compose := a.Compose()
			tmpl, err := tmplFlags.resolve(cmd.Context(), compose)
			if err != nil {
				return err
			}
			result, err := compose.PlanLoad.Execute(cmd.Context(), composeusecase.PlanLoadInput{
				Envs:         args,
				Template:     tmpl,
				OnlyTemplate: tmplFlags.only,
				Target:       out,
			})
			if err != nil {
				return presenter.ComposeError(err)
			}
			return a.Presenter().Plan(result.Plan, tmpl, result.Target, toFile)
		},
	}
	planCmd.Flags().StringVar(&out, planFlagOut, "", "arquivo de destino avaliado; sem ele, avalia o load no terminal")
	tmplFlags = templateBind(planCmd)
	planCmd.Flags().Bool(presenter.JSONFlag, true, "saída em JSON (sempre ligada)")
	return presenter.AlwaysJSON(planCmd)
}
