package usecase

import (
	"context"
	"fmt"
	"io"
)

const (
	SignalExitBase  = 128
	UnknownExitCode = 1
)

type ProcessSpec struct {
	Name    string
	Args    []string
	Environ []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type ProcessStatus struct {
	Code     int
	Signal   int
	Signaled bool
}

type RunCommandInput struct {
	Command []string
	Environ []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type RunCommandResult struct {
	ExitCode int
}

type RunCommand struct {
	processes ProcessRunner
}

func NewRunCommand(processes ProcessRunner) *RunCommand {
	return &RunCommand{processes: processes}
}

func (uc *RunCommand) Execute(_ context.Context, in RunCommandInput) (RunCommandResult, error) {
	if len(in.Command) == 0 {
		return RunCommandResult{}, ErrNoCommand
	}
	status, err := uc.processes.Run(ProcessSpec{
		Name:    in.Command[0],
		Args:    in.Command[1:],
		Environ: in.Environ,
		Stdin:   in.Stdin,
		Stdout:  in.Stdout,
		Stderr:  in.Stderr,
	})
	if err != nil {
		return RunCommandResult{}, fmt.Errorf("executar %s: %w", in.Command[0], err)
	}
	return RunCommandResult{ExitCode: exitCode(status)}, nil
}

func exitCode(status ProcessStatus) int {
	switch {
	case status.Signaled:
		return SignalExitBase + status.Signal
	case status.Code >= 0:
		return status.Code
	default:
		return UnknownExitCode
	}
}
