package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
)

func newUnsetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <env> KEY...",
		Short: "Remove chaves de uma env",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			keys := args[1:]
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			var removed []string
			_, err = v.Modify(cmd.Context(), name, func(env *vault.Env) error {
				removed = env.Unset(keys...)
				return nil
			})
			if err != nil {
				return err
			}
			if len(removed) == 0 {
				app.Infof("nenhuma chave removida em %q", name)
				return nil
			}
			app.Infof("removida(s) de %q: %s", name, strings.Join(removed, ", "))
			return nil
		},
	}
}
