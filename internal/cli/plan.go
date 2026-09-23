package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/gitignore"
	"github.com/giovalgas/envault/internal/merge"
	"github.com/giovalgas/envault/internal/vault"
)

const (
	planDefaultOut = ".env"
	planFlagOut    = "out"
)

type planTarget struct {
	Path       string           `json:"path"`
	Exists     bool             `json:"exists"`
	Gitignored gitignore.Status `json:"gitignored"`
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
			plan, source, err := planResolve(cmd.Context(), app, args, tmplFlags)
			if err != nil {
				return err
			}
			target, err := planInspectTarget(out)
			if err != nil {
				return err
			}
			return writeJSON(app.Stdout, planBuildEnvelope(plan, source, target))
		},
	}
	planCmd.Flags().StringVar(&out, planFlagOut, planDefaultOut, "arquivo de destino avaliado")
	tmplFlags = templateBind(planCmd)
	planCmd.Flags().Bool(jsonFlag, true, "saída em JSON (sempre ligada)")
	return alwaysJSON(planCmd)
}

func planResolve(ctx context.Context, app *App, names []string, tmplFlags *templateFlags) (merge.Plan, templateSource, error) {
	source := templateSource{}
	opts := merge.Options{}
	if tmplFlags != nil {
		resolved, err := tmplFlags.resolve()
		if err != nil {
			return merge.Plan{}, templateSource{}, err
		}
		source = resolved
		opts = tmplFlags.options()
	}
	envs, err := planLoadEnvs(ctx, app, names)
	if err != nil {
		return merge.Plan{}, templateSource{}, err
	}
	return merge.Resolve(envs, source.Template, opts), source, nil
}

func planLoadEnvs(ctx context.Context, app *App, names []string) ([]vault.Env, error) {
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, dup := seen[name]; dup {
			return nil, usageError(fmt.Errorf("env %q repetida", name))
		}
		seen[name] = struct{}{}
	}
	v, err := app.OpenVault()
	if err != nil {
		return nil, err
	}
	snapshot, err := v.Load(ctx)
	if err != nil {
		return nil, err
	}
	envs := make([]vault.Env, 0, len(names))
	for _, name := range names {
		env, ok := snapshot.Envs[name]
		if !ok {
			return nil, fmt.Errorf("%w: %q", vault.ErrEnvNotFound, name)
		}
		env.Name = name
		envs = append(envs, env.Clone())
	}
	return envs, nil
}

func planInspectTarget(path string) (planTarget, error) {
	target := planTarget{Path: path}
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		target.Exists = true
	case !errors.Is(err, fs.ErrNotExist):
		return planTarget{}, fmt.Errorf("inspecionar destino %s: %w", path, err)
	}
	target.Gitignored = gitignore.IsIgnored(path)
	return target, nil
}

func planBuildEnvelope(plan merge.Plan, source templateSource, target planTarget) planEnvelope {
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
