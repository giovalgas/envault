package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/giovalgas/envault/internal/vault"
)

const (
	execFlagEnv        = "env"
	execSignalExitBase = 128
)

func newExecCmd(app *App) *cobra.Command {
	var names []string
	var tmplFlags *templateFlags
	execCmd := &cobra.Command{
		Use:   "exec -e <env>[,<env>...] [flags] -- <cmd> [args...]",
		Short: "Roda um comando com as variáveis combinadas no ambiente, sem criar arquivo",
		Long: "exec combina as envs de -e na ordem dada (a última vence), aplica o template e roda o comando " +
			"com essas variáveis sobre o ambiente atual. O código de saída do comando é propagado.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(names) == 0 {
				return usageError(fmt.Errorf("informe ao menos uma env com -e"))
			}
			plan, _, err := planResolve(cmd.Context(), app, names, tmplFlags)
			if err != nil {
				return err
			}
			if len(plan.Missing) > 0 {
				app.Infof("aviso: chaves do template sem valor: %s", strings.Join(plan.Missing, ", "))
			}
			return execChild(app, args, execEnviron(os.Environ(), plan.Pairs()))
		},
	}
	execCmd.Flags().SetInterspersed(false)
	execCmd.Flags().StringSliceVarP(&names, execFlagEnv, "e", nil, "envs a combinar, separadas por vírgula ou com -e repetido")
	tmplFlags = templateBind(execCmd)
	return execCmd
}

func execChild(app *App, args []string, environ []string) error {
	child := exec.Command(args[0], args[1:]...)
	child.Env = environ
	child.Stdin = app.Stdin
	child.Stdout = app.Stdout
	child.Stderr = app.Stderr
	err := child.Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return withExitCode(execExitCode(exitErr), nil)
	}
	return fmt.Errorf("executar %s: %w", args[0], err)
}

func execExitCode(exitErr *exec.ExitError) int {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return execSignalExitBase + int(status.Signal())
	}
	if code := exitErr.ExitCode(); code >= 0 {
		return code
	}
	return ExitError
}

func execEnviron(base []string, vars []vault.Var) []string {
	override := make(map[string]struct{}, len(vars))
	for _, v := range vars {
		override[execEnvKey(v.Key)] = struct{}{}
	}
	environ := make([]string, 0, len(base)+len(vars))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := override[execEnvKey(key)]; replaced {
			continue
		}
		environ = append(environ, entry)
	}
	for _, v := range vars {
		environ = append(environ, v.Key+"="+v.Value)
	}
	return environ
}

func execEnvKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
