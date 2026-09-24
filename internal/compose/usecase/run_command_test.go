package usecase

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestRunCommandPassesSpec(t *testing.T) {
	processes := &fakeProcesses{}
	in, out, errOut := strings.NewReader("entrada"), &bytes.Buffer{}, &bytes.Buffer{}
	result, err := NewRunCommand(processes).Execute(context.Background(), RunCommandInput{
		Command: []string{"prog", "-a", "b"},
		Environ: []string{"X=1"},
		Stdin:   in,
		Stdout:  out,
		Stderr:  errOut,
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
	spec := processes.spec
	if spec.Name != "prog" || !slices.Equal(spec.Args, []string{"-a", "b"}) || !slices.Equal(spec.Environ, []string{"X=1"}) {
		t.Fatalf("spec = %+v", spec)
	}
	if spec.Stdin != in || spec.Stdout != out || spec.Stderr != errOut {
		t.Fatal("streams not passed to the process")
	}
}

func TestRunCommandExitCodes(t *testing.T) {
	cases := []struct {
		status ProcessStatus
		want   int
	}{
		{ProcessStatus{Code: 7}, 7},
		{ProcessStatus{Signaled: true, Signal: 15}, SignalExitBase + 15},
		{ProcessStatus{Code: -1}, UnknownExitCode},
	}
	for _, tc := range cases {
		result, err := NewRunCommand(&fakeProcesses{status: tc.status}).Execute(context.Background(), RunCommandInput{Command: []string{"prog"}})
		if err != nil || result.ExitCode != tc.want {
			t.Errorf("%+v: result = %+v, %v", tc.status, result, err)
		}
	}
}

func TestRunCommandErrors(t *testing.T) {
	processes := &fakeProcesses{}
	if _, err := NewRunCommand(processes).Execute(context.Background(), RunCommandInput{}); !errors.Is(err, ErrNoCommand) || processes.calls != 0 {
		t.Fatalf("empty err = %v calls %d", err, processes.calls)
	}
	_, err := NewRunCommand(&fakeProcesses{err: errBoom}).Execute(context.Background(), RunCommandInput{Command: []string{"prog"}})
	if !errors.Is(err, errBoom) || err.Error() != "executar prog: boom" {
		t.Fatalf("start err = %v", err)
	}
}
