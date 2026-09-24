package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
)

func NewUnsetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <env> KEY...",
		Short: "Remove chaves de uma env",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			keys := args[1:]
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			removed, err := uc.UnsetKeys.Execute(cmd.Context(), name, keys)
			if err != nil {
				return err
			}
			return a.Presenter().KeysUnset(name, removed)
		},
	}
}
