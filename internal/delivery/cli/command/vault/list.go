package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

func NewListCmd(a *app.App) *cobra.Command {
	var search, tag string
	var asJSON bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista as envs do cofre, sem valores",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			filtered, err := uc.ListEnvs.Execute(cmd.Context(), vaultusecase.ListEnvsQuery{Search: search, Tag: tag})
			if err != nil {
				return err
			}
			return a.Presenter().EnvList(filtered, asJSON)
		},
	}
	listCmd.Flags().StringVar(&search, "search", "", "filtra por nome, descrição, tag ou nome de chave")
	listCmd.Flags().StringVar(&tag, "tag", "", "filtra por tag exata")
	listCmd.Flags().BoolVar(&asJSON, presenter.JSONFlag, false, "saída em JSON")
	return listCmd
}
