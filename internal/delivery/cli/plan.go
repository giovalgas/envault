package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	composedomain "github.com/giovalgas/envault/internal/compose/domain"
	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

const (
	planDefaultOut = ".env"
	planFlagOut    = "out"
)

type planTarget struct {
	Path       string                        `json:"path"`
	Exists     bool                          `json:"exists"`
	Gitignored composedomain.GitignoreStatus `json:"gitignored"`
}

type planTemplate struct {
	Path  *string `json:"path"`
	Found bool    `json:"found"`
}

type planKey struct {
	Key     string   `json:"key"`
	From    *string  `json:"from"`
	Shadows []string `json:"shadows"`
	Default bool     `json:"default"`
}

type planEnvelope struct {
	SchemaVersion int          `json:"schema_version"`
	Envs          []string     `json:"envs"`
	Target        planTarget   `json:"target"`
	Template      planTemplate `json:"template"`
	Keys          []planKey    `json:"keys"`
	Conflicts     []string     `json:"conflicts"`
	Missing       []string     `json:"missing"`
	Extra         []string     `json:"extra"`
}

func newPlanCmd(app *App) *cobra.Command {
	var out string
	var tmplFlags *templateFlags
	planCmd := &cobra.Command{
		Use:   "plan <env>...",
		Short: "Mostra em JSON o que load gravaria, sem gravar nada e sem valores",
		Long: "plan combina as envs na ordem dada (a última vence), aplica o template e descreve o resultado em JSON: " +
			"chaves, origem, conflitos, faltando e extras. Nunca grava arquivo e nunca inclui valores.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := tmplFlags.resolve()
			if err != nil {
				return err
			}
			result, err := app.Compose().PlanLoad.Execute(cmd.Context(), composeusecase.PlanLoadInput{
				Envs:     args,
				Template: source.Template,
				Options:  tmplFlags.options(),
				Target:   out,
			})
			if err != nil {
				return composeError(err)
			}
			return writeJSON(app.Stdout, planBuildEnvelope(result.Plan, source, planTargetOf(result.Target)))
		},
	}
	planCmd.Flags().StringVar(&out, planFlagOut, planDefaultOut, "arquivo de destino avaliado")
	tmplFlags = templateBind(planCmd)
	planCmd.Flags().Bool(jsonFlag, true, "saída em JSON (sempre ligada)")
	return alwaysJSON(planCmd)
}

func composeError(err error) error {
	var duplicate *composeusecase.DuplicateEnvError
	var missing *composeusecase.EnvNotFoundError
	switch {
	case errors.As(err, &duplicate):
		return usageError(err)
	case errors.As(err, &missing):
		return fmt.Errorf("%w: %q", vault.ErrEnvNotFound, missing.Name)
	}
	return err
}

func planTargetOf(target composedomain.Target) planTarget {
	return planTarget{Path: target.Path, Exists: target.Exists, Gitignored: target.Gitignored}
}

func planBuildEnvelope(plan composedomain.Plan, source templateSource, target planTarget) planEnvelope {
	keys := make([]planKey, len(plan.Vars))
	for i, resolved := range plan.Vars {
		key := planKey{Key: resolved.Key, Shadows: resolved.ShadowNames(), Default: resolved.Default}
		if resolved.From != "" {
			from := resolved.From
			key.From = &from
		}
		keys[i] = key
	}
	tmpl := planTemplate{Found: source.Found}
	if source.Path != "" {
		path := source.Path
		tmpl.Path = &path
	}
	return planEnvelope{
		SchemaVersion: SchemaVersion,
		Envs:          plan.Envs,
		Target:        target,
		Template:      tmpl,
		Keys:          keys,
		Conflicts:     plan.Conflicts,
		Missing:       plan.Missing,
		Extra:         plan.Extra,
	}
}
