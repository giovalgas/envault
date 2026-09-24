package compose

import (
	"slices"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

const shellFlagName = "shell"

var shellDialects = composeusecase.ShellDialects()

func NewShellCmd(a *app.App) *cobra.Command {
	var dialect string
	shellCmd := &cobra.Command{
		Use:   "shell <env>... [--shell bash|zsh|fish]",
		Short: "Imprime as variáveis combinadas como comandos de export para eval",
		Long: "shell combina as envs na ordem dada (a última vence) e imprime uma linha de export por variável, " +
			"no dialeto do shell pedido, para uso com eval. Sem --shell, usa o shell de $SHELL quando reconhecido, senão bash.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chosen, err := shellChoose(a, dialect, cmd.Flags().Changed(shellFlagName))
			if err != nil {
				return err
			}
			rendered, err := a.Compose().RenderShell.Execute(cmd.Context(), composeusecase.RenderShellInput{Envs: args, Dialect: chosen})
			if err != nil {
				return presenter.ComposeError(err)
			}
			return a.Presenter().Script(rendered.Script)
		},
	}
	shellCmd.Flags().StringVar(&dialect, shellFlagName, "", "dialeto: bash, zsh ou fish")
	return shellCmd
}

func shellChoose(a *app.App, dialect string, explicit bool) (string, error) {
	if explicit {
		if !slices.Contains(shellDialects, dialect) {
			return "", presenter.UnsupportedShell(dialect, shellDialects)
		}
		return dialect, nil
	}
	return a.DetectShell(), nil
}
