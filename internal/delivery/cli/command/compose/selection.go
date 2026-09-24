package compose

import (
	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

func NewSelectionCmd(a *app.App) *cobra.Command {
	var asJSON bool
	selectionCmd := &cobra.Command{
		Use:   "selection",
		Short: "Mostra as envs selecionadas na TUI, na ordem de precedência, sem valores",
		Long:  "Mostra a seleção global gravada pela TUI em selection.json, na ordem de precedência: a última vence conflitos. Envs gravadas que não existem mais no cofre saem em missing.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := a.Compose().GetSelection.Execute(cmd.Context())
			if err != nil {
				return err
			}
			return a.Presenter().Selection(result, asJSON)
		},
	}
	selectionCmd.Flags().BoolVar(&asJSON, presenter.JSONFlag, false, "saída em JSON")
	return selectionCmd
}
