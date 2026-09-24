package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/giovalgas/envault/internal/compose/usecase"
)

type Runner struct{}

var (
	_ usecase.ProcessRunner = Runner{}
	_ usecase.Environment   = Runner{}
)

func New() Runner {
	return Runner{}
}

func (Runner) Environ() []string {
	return os.Environ()
}

func (Runner) Run(spec usecase.ProcessSpec) (usecase.ProcessStatus, error) {
	child := exec.Command(spec.Name, spec.Args...)
	child.Env = spec.Environ
	child.Stdin = spec.Stdin
	child.Stdout = spec.Stdout
	child.Stderr = spec.Stderr
	err := child.Run()
	if err == nil {
		return usecase.ProcessStatus{}, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return statusOf(exitErr), nil
	}
	return usecase.ProcessStatus{}, err
}

func statusOf(exitErr *exec.ExitError) usecase.ProcessStatus {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return usecase.ProcessStatus{Signaled: true, Signal: int(status.Signal())}
	}
	return usecase.ProcessStatus{Code: exitErr.ExitCode()}
}
