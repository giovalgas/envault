package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
)

func NewCopyCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "copy <origem> <destino>",
		Short: "Duplica uma env",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			if _, err := uc.CopyEnv.Execute(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			return a.Presenter().EnvCopied(args[0], args[1])
		},
	}
}
