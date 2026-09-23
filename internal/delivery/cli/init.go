package cli

import (
	"github.com/spf13/cobra"
)

func newInitCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Cria o diretório, a chave e o cofre vazio, se ainda não existirem",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			result, err := uc.InitVault.Execute(cmd.Context())
			if err != nil {
				return err
			}
			if result.Created {
				app.Infof("cofre criado em %s", result.Location)
			} else {
				app.Infof("cofre já inicializado em %s", result.Location)
			}
			return nil
		},
	}
}
