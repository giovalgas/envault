package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <env> <chave>",
		Short: "Imprime o valor de uma chave. Para humanos e scripts",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			value, err := uc.GetValue.Execute(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprintln(app.Stdout, value)
			return nil
		},
	}
}
