package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
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
			source, err := tmplFlags.resolve()
			if err != nil {
				return err
			}
			result, err := app.Compose().ExecWithEnvs.Execute(cmd.Context(), composeusecase.ExecWithEnvsInput{
				Envs:     names,
				Template: source.Template,
				Options:  tmplFlags.options(),
				Environ:  os.Environ(),
			})
			if err != nil {
				return composeError(err)
			}
			if len(result.Plan.Missing) > 0 {
				app.Infof("aviso: chaves do template sem valor: %s", strings.Join(result.Plan.Missing, ", "))
			}
			return execChild(app, args, result.Environ)
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
