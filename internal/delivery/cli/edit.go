package cli

import (
	"github.com/spf13/cobra"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func newEditCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <env>",
		Short: "Edita uma env no $VISUAL ou $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			uc, err := app.Vault()
			if err != nil {
				return err
			}
			result, err := uc.EditEnv.Execute(cmd.Context(), name)
			if err != nil {
				return editorExitCode(err)
			}
			if !result.Changed {
				app.Infof("nada mudou em %q", name)
				return nil
			}
			app.Infof("env %q atualizada:", name)
			editReportDiff(app, result.Diff)
			return nil
		},
	}
}

func editReportDiff(app *App, diff vault.Diff) {
	for _, line := range diff.Lines() {
		app.Infof("  %s", line)
	}
}
