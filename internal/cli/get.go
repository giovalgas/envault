package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var errGetKeyNotFound = errors.New("chave não encontrada")

func newGetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <env> <chave>",
		Short: "Imprime o valor de uma chave. Para humanos e scripts",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			env, err := v.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			value, ok := env.Lookup(args[1])
			if !ok {
				return fmt.Errorf("%w: %q em %q", errGetKeyNotFound, args[1], args[0])
			}
			fmt.Fprintln(app.Stdout, value)
			return nil
		},
	}
}
