package compose

import (
	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

const execFlagEnv = "env"

func NewExecCmd(a *app.App) *cobra.Command {
	var names []string
	var tmplFlags *templateFlags
	execCmd := &cobra.Command{
		Use:   "exec -e <env>[,<env>...] [flags] -- <cmd> [args...]",
		Short: "Roda um comando com as variáveis combinadas no ambiente, sem criar arquivo",
		Long: "exec combina as envs de -e na ordem dada (a última vence), aplica o template e roda o comando " +
			"com essas variáveis sobre o ambiente atual. O código de saída do comando é propagado.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(names) == 0 {
				return presenter.NoEnvGiven()
			}
			compose := a.Compose()
			tmpl, err := tmplFlags.resolve(cmd.Context(), compose)
			if err != nil {
				return err
			}
			result, err := compose.ExecWithEnvs.Execute(cmd.Context(), composeusecase.ExecWithEnvsInput{
				Envs:         names,
				Template:     tmpl,
				OnlyTemplate: tmplFlags.only,
			})
			if err != nil {
				return presenter.ComposeError(err)
			}
			if err := a.Presenter().ExecMissing(result.Plan.Missing); err != nil {
				return err
			}
			return execChild(cmd, a, compose, args, result.Environ)
		},
	}
	execCmd.Flags().SetInterspersed(false)
	execCmd.Flags().StringSliceVarP(&names, execFlagEnv, "e", nil, "envs a combinar, separadas por vírgula ou com -e repetido")
	tmplFlags = templateBind(execCmd)
	return execCmd
}

func execChild(cmd *cobra.Command, a *app.App, compose app.ComposeUseCases, args, environ []string) error {
	result, err := compose.RunCommand.Execute(cmd.Context(), composeusecase.RunCommandInput{
		Command: args,
		Environ: environ,
		Stdin:   a.Stdin,
		Stdout:  a.Stdout,
		Stderr:  a.Stderr,
	})
	if err != nil {
		return err
	}
	return presenter.ChildExited(result)
}
