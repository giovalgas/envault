package cli

import (
	"github.com/spf13/cobra"
)

func newRenameCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <atual> <novo>",
		Short: "Renomeia uma env",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			if _, err := uc.RenameEnv.Execute(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			return app.Infof("%q renomeada para %q", args[0], args[1])
		},
	}
}
