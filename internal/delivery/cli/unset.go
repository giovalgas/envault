package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newUnsetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <env> KEY...",
		Short: "Remove chaves de uma env",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			keys := args[1:]
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			removed, err := uc.UnsetKeys.Execute(cmd.Context(), name, keys)
			if err != nil {
				return err
			}
			if len(removed) == 0 {
				return app.Infof("nenhuma chave removida em %q", name)
			}
			return app.Infof("removida(s) de %q: %s", name, strings.Join(removed, ", "))
		},
	}
}
