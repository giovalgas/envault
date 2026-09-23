package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/dotenv"
	"github.com/giovalgas/envault/internal/gitignore"
	"github.com/giovalgas/envault/internal/merge"
	"github.com/giovalgas/envault/internal/vault"
)

const (
	loadFlagForce       = "force"
	loadFlagMerge       = "merge"
	loadModeCreated     = "created"
	loadModeOverwritten = "overwritten"
	loadModeMerged      = "merged"
)

type loadEnvelope struct {
	planEnvelope
	Mode string `json:"mode"`
}

type loadOptions struct {
	out       string
	force     bool
	merge     bool
	asJSON    bool
	tmplFlags *templateFlags
}

func newLoadCmd(app *App) *cobra.Command {
	opts := &loadOptions{}
	loadCmd := &cobra.Command{
		Use:   "load <env>...",
		Short: "Grava o .env combinando as envs na ordem dada (a última vence)",
		Long: "load combina as envs, aplica o template e grava o destino. Recusa quando o destino já existe, " +
			"a menos que --force (substitui) ou --merge (mantém chaves locais e atualiza as das envs) seja passado. " +
			"Nunca altera o .gitignore: só avisa quando o destino não está coberto por ele.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.force && opts.merge {
				return usageError(fmt.Errorf("--%s e --%s não podem ser usados juntos", loadFlagForce, loadFlagMerge))
			}
			plan, source, err := planResolve(cmd.Context(), app, args, opts.tmplFlags)
			if err != nil {
				return err
			}
			return loadRun(app, opts, plan, source)
		},
	}
	loadCmd.Flags().StringVar(&opts.out, planFlagOut, planDefaultOut, "arquivo de destino")
	loadCmd.Flags().BoolVar(&opts.force, loadFlagForce, false, "substitui o destino se ele existir")
	loadCmd.Flags().BoolVar(&opts.merge, loadFlagMerge, false, "mescla com o destino existente, preservando a ordem e as chaves locais")
	opts.tmplFlags = templateBind(loadCmd)
	loadCmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "saída em JSON")
	return loadCmd
}

func loadRun(app *App, opts *loadOptions, plan merge.Plan, source templateSource) error {
	before, err := planInspectTarget(opts.out)
	if err != nil {
		return err
	}
	if before.Exists && !opts.force && !opts.merge {
		return fmt.Errorf("%w: %s (use --%s ou --%s)", ErrTargetExists, opts.out, loadFlagForce, loadFlagMerge)
	}
	content, mode, err := loadContent(opts, before.Exists, plan)
	if err != nil {
		return err
	}
	if err := loadWriteFile(opts.out, dotenv.Format(content)); err != nil {
		return err
	}
	after := planTarget{Path: opts.out, Exists: before.Exists, Gitignored: gitignore.IsIgnored(opts.out)}
	loadReport(app, opts, plan, after, mode, len(content.Vars))
	if !opts.asJSON {
		return nil
	}
	return writeJSON(app.Stdout, loadEnvelope{planEnvelope: planBuildEnvelope(plan, source, after), Mode: mode})
}

func loadContent(opts *loadOptions, exists bool, plan merge.Plan) (vault.Env, string, error) {
	switch {
	case !exists:
		return vault.Env{Vars: plan.Pairs()}, loadModeCreated, nil
	case opts.force:
		return vault.Env{Vars: plan.Pairs()}, loadModeOverwritten, nil
	}
	data, err := os.ReadFile(opts.out)
	if err != nil {
		return vault.Env{}, "", fmt.Errorf("ler destino %s: %w", opts.out, err)
	}
	current, err := dotenv.Parse(data)
	if err != nil {
		return vault.Env{}, "", fmt.Errorf("mesclar com %s: %w", opts.out, err)
	}
	merged := vault.Env{
		Description: current.Description,
		Tags:        current.Tags,
		Vars:        merge.MergeVars(current.Vars, plan.Pairs()),
	}
	return merged, loadModeMerged, nil
}

func loadReport(app *App, opts *loadOptions, plan merge.Plan, target planTarget, mode string, written int) {
	if !opts.asJSON {
		app.Infof("%s %s com %d variáveis de %s.", loadModeVerb(mode), opts.out, written, strings.Join(plan.Envs, ", "))
		if len(plan.Conflicts) > 0 {
			app.Infof("Conflitos resolvidos pela última env: %s.", strings.Join(plan.Conflicts, ", "))
		}
	}
	if len(plan.Missing) > 0 {
		app.Infof("aviso: chaves do template sem valor, não gravadas: %s", strings.Join(plan.Missing, ", "))
	}
	if target.Gitignored == gitignore.NotIgnored {
		app.Infof("aviso: %s não está coberto pelo .gitignore; adicione-o para não versionar segredos", opts.out)
	}
}

func loadModeVerb(mode string) string {
	switch mode {
	case loadModeOverwritten:
		return "Substituído"
	case loadModeMerged:
		return "Mesclado"
	default:
		return "Gravado"
	}
}

func loadWriteFile(path string, data []byte) error {
	target, err := loadResolveTarget(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".envault-*")
	if err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	committed = true
	return nil
}

func loadResolveTarget(path string) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return path, nil
	case err != nil:
		return "", fmt.Errorf("inspecionar destino %s: %w", path, err)
	case info.IsDir():
		return "", fmt.Errorf("%w: destino %s é um diretório", ErrValidation, path)
	case info.Mode()&fs.ModeSymlink == 0:
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolver link %s: %w", path, err)
	}
	return resolved, nil
}
