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
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			created, err := v.Init(cmd.Context())
			if err != nil {
				return err
			}
			if created {
				app.Infof("cofre criado em %s", v.Paths().Dir)
			} else {
				app.Infof("cofre já inicializado em %s", v.Paths().Dir)
			}
			return nil
		},
	}
}
