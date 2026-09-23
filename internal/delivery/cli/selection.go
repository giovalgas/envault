package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
)

type selectionEnvelope struct {
	SchemaVersion int        `json:"schema_version"`
	Envs          []string   `json:"envs"`
	Missing       []string   `json:"missing"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

func newSelectionCmd(app *App) *cobra.Command {
	var asJSON bool
	selectionCmd := &cobra.Command{
		Use:   "selection",
		Short: "Mostra as envs selecionadas na TUI, na ordem de precedência, sem valores",
		Long:  "Mostra a seleção global gravada pela TUI em selection.json, na ordem de precedência: a última vence conflitos. Envs gravadas que não existem mais no cofre saem em missing.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := app.Compose().GetSelection.Execute(cmd.Context())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(app.Stdout, selectionBuildEnvelope(result))
			}
			return selectionPrintHuman(app, result)
		},
	}
	selectionCmd.Flags().BoolVar(&asJSON, jsonFlag, false, "saída em JSON")
	return selectionCmd
}

func selectionBuildEnvelope(result composeusecase.GetSelectionResult) selectionEnvelope {
	envelope := selectionEnvelope{
		SchemaVersion: SchemaVersion,
		Envs:          listNonNil(result.Selection.Envs),
		Missing:       listNonNil(result.Missing),
	}
	if result.Selection.Saved() {
		updatedAt := result.Selection.UpdatedAt
		envelope.UpdatedAt = &updatedAt
	}
	return envelope
}

func selectionPrintHuman(app *App, result composeusecase.GetSelectionResult) error {
	if len(result.Missing) > 0 {
		if err := app.Infof("fora do cofre, ignoradas: %s", strings.Join(result.Missing, ", ")); err != nil {
			return err
		}
	}
	if len(result.Selection.Envs) == 0 {
		return app.Infof("nenhuma env selecionada")
	}
	var b strings.Builder
	for _, name := range result.Selection.Envs {
		fmt.Fprintf(&b, "%s\n", name)
	}
	_, err := io.WriteString(app.Stdout, b.String())
	return err
}
