package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
)

func NewGetCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <env> <chave>",
		Short: "Imprime o valor de uma chave. Para humanos e scripts",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			value, err := uc.GetValue.Execute(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return a.Presenter().Value(value)
		},
	}
}
