package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

func NewShowCmd(a *app.App) *cobra.Command {
	var asJSON bool
	showCmd := &cobra.Command{
		Use:   "show <env>",
		Short: "Mostra metadados e nomes das chaves de uma env, sem valores",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			env, err := uc.ShowEnv.Execute(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.Presenter().Env(env, asJSON)
		},
	}
	showCmd.Flags().BoolVar(&asJSON, presenter.JSONFlag, false, "saída em JSON")
	return showCmd
}
