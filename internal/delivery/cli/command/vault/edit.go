package vault

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

func NewEditCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <env>",
		Short: "Edita uma env no $VISUAL ou $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			uc, err := a.Vault()
			if err != nil {
				return err
			}
			result, err := uc.EditEnv.Execute(cmd.Context(), name)
			if err != nil {
				return presenter.EditorError(err)
			}
			return a.Presenter().EnvEdited(name, result)
		},
	}
}
