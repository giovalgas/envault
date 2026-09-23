package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
)

const (
	shellFlagName = "shell"
	shellBash     = composeusecase.ShellBash
	shellZsh      = composeusecase.ShellZsh
	shellFish     = composeusecase.ShellFish
	shellEnvVar   = "SHELL"
)

var shellDialects = composeusecase.ShellDialects()

func newShellCmd(app *App) *cobra.Command {
	var dialect string
	shellCmd := &cobra.Command{
		Use:   "shell <env>... [--shell bash|zsh|fish]",
		Short: "Imprime as variáveis combinadas como comandos de export para eval",
		Long: "shell combina as envs na ordem dada (a última vence) e imprime uma linha de export por variável, " +
			"no dialeto do shell pedido, para uso com eval. Sem --shell, usa o shell de $SHELL quando reconhecido, senão bash.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chosen, err := shellChoose(dialect, cmd.Flags().Changed(shellFlagName))
			if err != nil {
				return err
			}
			rendered, err := app.Compose().RenderShell.Execute(cmd.Context(), composeusecase.RenderShellInput{Envs: args, Dialect: chosen})
			if err != nil {
				return composeError(err)
			}
			_, err = fmt.Fprint(app.Stdout, rendered.Script)
			return err
		},
	}
	shellCmd.Flags().StringVar(&dialect, shellFlagName, "", "dialeto: bash, zsh ou fish")
	return shellCmd
}

func shellChoose(dialect string, explicit bool) (string, error) {
	if explicit {
		if !slices.Contains(shellDialects, dialect) {
			return "", usageError(fmt.Errorf("shell %q não suportado, use %s", dialect, strings.Join(shellDialects, ", ")))
		}
		return dialect, nil
	}
	return shellDetect(), nil
}

func shellDetect() string {
	detected := strings.TrimSuffix(filepath.Base(os.Getenv(shellEnvVar)), ".exe")
	if slices.Contains(shellDialects, detected) {
		return detected
	}
	return shellBash
}
