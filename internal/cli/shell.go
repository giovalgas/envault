package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
)

const (
	shellFlagName = "shell"
	shellBash     = "bash"
	shellZsh      = "zsh"
	shellFish     = "fish"
	shellEnvVar   = "SHELL"
)

var shellDialects = []string{shellBash, shellZsh, shellFish}

var (
	shellPOSIXQuoter = strings.NewReplacer(`'`, `'\''`)
	shellFishQuoter  = strings.NewReplacer(`\`, `\\`, `'`, `\'`)
)

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
			plan, _, err := planResolve(cmd.Context(), app, args, nil)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(app.Stdout, shellRender(chosen, plan.Pairs()))
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
	detected := strings.TrimSuffix(filepath.Base(os.Getenv(shellEnvVar)), ".exe")
	if slices.Contains(shellDialects, detected) {
		return detected, nil
	}
	return shellBash, nil
}

func shellRender(dialect string, vars []vault.Var) string {
	var b strings.Builder
	for _, v := range vars {
		if dialect == shellFish {
			fmt.Fprintf(&b, "set -gx %s '%s'\n", v.Key, shellFishQuoter.Replace(v.Value))
			continue
		}
		fmt.Fprintf(&b, "export %s='%s'\n", v.Key, shellPOSIXQuoter.Replace(v.Value))
	}
	return b.String()
}
