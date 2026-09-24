package compose

import (
	"context"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
)

const (
	loadFlagForce     = "force"
	loadFlagMerge     = "merge"
	loadFlagClipboard = "clipboard"
)

type loadOptions struct {
	out       string
	force     bool
	merge     bool
	clipboard bool
	asJSON    bool
	tmplFlags *templateFlags
}

func NewLoadCmd(a *app.App) *cobra.Command {
	opts := &loadOptions{}
	loadCmd := &cobra.Command{
		Use:   "load <env>...",
		Short: "Exporta no terminal atual as envs combinadas na ordem dada (a última vence), ou grava com --out ou copia com --clipboard",
		Long: "load combina as envs e aplica o template. Sem --out, exporta as variáveis no shell atual pelo wrapper " +
			"de shell-init, sem gravar .env e sem imprimir valores; sem o wrapper, sai com 2. Com --out, grava o arquivo " +
			"e recusa quando ele já existe, a menos que --force (substitui) ou --merge (mantém chaves locais e atualiza " +
			"as das envs) seja passado. Com --clipboard, copia para o clipboard o mesmo conteúdo que --out gravaria, sem " +
			"tocar em arquivo. Nunca altera o .gitignore: só avisa quando o arquivo não está coberto por ele.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toFile, err := outRequested(cmd, opts.out)
			if err != nil {
				return err
			}
			if err := opts.validate(toFile); err != nil {
				return err
			}
			switch {
			case opts.clipboard:
				return loadClipboard(cmd.Context(), a, opts, args)
			case toFile:
				return loadFile(cmd.Context(), a, opts, args)
			}
			return loadShell(cmd.Context(), a, opts, args)
		},
	}
	loadCmd.Flags().StringVar(&opts.out, planFlagOut, "", "arquivo de destino; sem ele, exporta no terminal pelo wrapper de shell-init")
	loadCmd.Flags().BoolVar(&opts.force, loadFlagForce, false, "substitui o arquivo de --out se ele existir")
	loadCmd.Flags().BoolVar(&opts.merge, loadFlagMerge, false, "mescla com o arquivo de --out existente, preservando a ordem e as chaves locais")
	loadCmd.Flags().BoolVar(&opts.clipboard, loadFlagClipboard, false, "copia para o clipboard o conteúdo do .env em vez de gravar arquivo")
	opts.tmplFlags = templateBind(loadCmd)
	loadCmd.Flags().BoolVar(&opts.asJSON, presenter.JSONFlag, false, "saída em JSON")
	return loadCmd
}

func (o *loadOptions) validate(toFile bool) error {
	if o.clipboard && toFile {
		return presenter.FlagsConflict(loadFlagClipboard, planFlagOut)
	}
	if o.force && o.merge {
		return presenter.FlagsConflict(loadFlagForce, loadFlagMerge)
	}
	if !toFile && (o.force || o.merge) {
		return presenter.FlagsNeedFlag(loadFlagForce, loadFlagMerge, planFlagOut)
	}
	return nil
}

func (o *loadOptions) existing() composeusecase.ExistingTarget {
	switch {
	case o.force:
		return composeusecase.ReplaceExisting
	case o.merge:
		return composeusecase.MergeExisting
	default:
		return composeusecase.RefuseExisting
	}
}

func loadShell(ctx context.Context, a *app.App, opts *loadOptions, args []string) error {
	export := a.ShellExport()
	if !export.Active() {
		return presenter.LoadNeedsWrapper(planFlagOut, export.Dialect)
	}
	compose := a.Compose()
	source, err := opts.tmplFlags.resolve(ctx, compose)
	if err != nil {
		return err
	}
	result, err := compose.LoadShellExports.Execute(ctx, composeusecase.LoadShellExportsInput{
		Envs:         args,
		Template:     source,
		OnlyTemplate: opts.tmplFlags.only,
		Dialect:      export.Dialect,
		ExportFile:   export.File,
	})
	if err != nil {
		return presenter.LoadShellError(err, planFlagOut, export.Dialect)
	}
	return a.Presenter().LoadedShell(result, source, opts.asJSON)
}

func loadFile(ctx context.Context, a *app.App, opts *loadOptions, args []string) error {
	compose := a.Compose()
	source, err := opts.tmplFlags.resolve(ctx, compose)
	if err != nil {
		return err
	}
	result, err := compose.LoadEnvFile.Execute(ctx, composeusecase.LoadEnvFileInput{
		Envs:         args,
		Template:     source,
		OnlyTemplate: opts.tmplFlags.only,
		Target:       opts.out,
		Existing:     opts.existing(),
	})
	if err != nil {
		return presenter.LoadFileError(err, opts.out, loadFlagForce, loadFlagMerge)
	}
	return a.Presenter().LoadedFile(result, source, opts.out, opts.asJSON)
}

func loadClipboard(ctx context.Context, a *app.App, opts *loadOptions, args []string) error {
	compose := a.Compose()
	source, err := opts.tmplFlags.resolve(ctx, compose)
	if err != nil {
		return err
	}
	result, err := compose.RenderEnvFile.Execute(ctx, composeusecase.RenderEnvFileInput{
		Envs:         args,
		Template:     source,
		OnlyTemplate: opts.tmplFlags.only,
	})
	if err != nil {
		return presenter.ComposeError(err)
	}
	if err := a.Clipboard(result.Content); err != nil {
		return presenter.ClipboardError(err)
	}
	return a.Presenter().LoadedClipboard(result, source, opts.asJSON)
}
