package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
)

func NewInitCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Cria o diretório, a chave e o cofre vazio, se ainda não existirem",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			result, err := uc.InitVault.Execute(cmd.Context())
			if err != nil {
				return err
			}
			return a.Presenter().VaultInitialized(result)
		},
	}
}
