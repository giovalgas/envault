package process

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/compose/usecase"
)

const helperEnv = "ENVAULT_TEST_PROCESS_HELPER"

func TestHelperProcess(*testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	switch args[0] {
	case "print":
		fmt.Print(os.Getenv(args[1]))
		os.Exit(0)
	case "exit":
		code, err := strconv.Atoi(args[1])
		if err != nil {
			os.Exit(98)
		}
		os.Exit(code)
	}
	os.Exit(99)
}

func helperSpec(mode ...string) usecase.ProcessSpec {
	return usecase.ProcessSpec{
		Name:    os.Args[0],
		Args:    append([]string{"-test.run=^TestHelperProcess$", "--"}, mode...),
		Environ: append(os.Environ(), helperEnv+"=1", "GORACE=atexit_sleep_ms=0", "ENVAULT_PROCESS_VALUE=injetado"),
	}
}

func TestRunPassesEnvironAndStreams(t *testing.T) {
	var out bytes.Buffer
	spec := helperSpec("print", "ENVAULT_PROCESS_VALUE")
	spec.Stdout = &out
	status, err := New().Run(spec)
	if err != nil || status != (usecase.ProcessStatus{}) {
		t.Fatalf("Run = %+v, %v", status, err)
	}
	if out.String() != "injetado" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestRunReportsExitCode(t *testing.T) {
	status, err := New().Run(helperSpec("exit", "7"))
	if err != nil || status.Code != 7 || status.Signaled {
		t.Fatalf("Run = %+v, %v", status, err)
	}
}

func TestRunStartFailure(t *testing.T) {
	_, err := New().Run(usecase.ProcessSpec{Name: "envault-comando-que-nao-existe"})
	if err == nil {
		t.Fatal("expected start error")
	}
}

func TestEnviron(t *testing.T) {
	t.Setenv("ENVAULT_PROCESS_ENVIRON", "sim")
	found := false
	for _, entry := range New().Environ() {
		if strings.HasPrefix(entry, "ENVAULT_PROCESS_ENVIRON=") {
			found = entry == "ENVAULT_PROCESS_ENVIRON=sim"
		}
	}
	if !found {
		t.Fatal("Environ does not reflect the process environment")
	}
}
