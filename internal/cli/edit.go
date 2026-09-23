package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/editor"
	"github.com/giovalgas/envault/internal/vault"
)

var errEditConcurrent = errors.New("a env mudou no cofre durante a edição; nada foi gravado")

func newEditCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <env>",
		Short: "Edita uma env no $VISUAL ou $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			v, err := app.OpenVault()
			if err != nil {
				return err
			}
			initial, err := v.Get(cmd.Context(), name)
			if err != nil {
				return err
			}
			result, err := editRunEditor(cmd.Context(), app, initial, false)
			if err != nil {
				return err
			}
			if !result.Changed {
				app.Infof("nada mudou em %q", name)
				return nil
			}
			if err := editApply(cmd.Context(), v, initial, result.Env); err != nil {
				return err
			}
			app.Infof("env %q atualizada:", name)
			editReportDiff(app, result.Diff)
			return nil
		},
	}
}

func editRunEditor(ctx context.Context, app *App, initial vault.Env, isNew bool) (editor.Result, error) {
	cfg, err := app.Config()
	if err != nil {
		return editor.Result{}, err
	}
	result, err := editor.Edit(ctx, initial, editor.Options{
		New:        isNew,
		RuntimeDir: cfg.RuntimeDir,
		Stdin:      app.Stdin,
		Stdout:     app.Stdout,
		Stderr:     app.Stderr,
	})
	if errors.Is(err, editor.ErrCanceled) {
		return editor.Result{}, withExitCode(ExitCanceled, err)
	}
	return result, err
}

func editApply(ctx context.Context, v *vault.Vault, initial, edited vault.Env) error {
	_, err := v.Modify(ctx, initial.Name, func(env *vault.Env) error {
		if !env.SameContent(initial) {
			return errEditConcurrent
		}
		env.Description = edited.Description
		env.Tags = edited.Tags
		env.Vars = edited.Vars
		return nil
	})
	return err
}

func editReportDiff(app *App, diff editor.Diff) {
	for _, line := range diff.Lines() {
		app.Infof("  %s", line)
	}
}
