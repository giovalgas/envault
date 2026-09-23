package cli

import (
	"github.com/spf13/cobra"
)

func newCopyCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "copy <origem> <destino>",
		Short: "Duplica uma env",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			if _, err := uc.CopyEnv.Execute(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			app.Infof("%q duplicada para %q", args[0], args[1])
			return nil
		},
	}
}
