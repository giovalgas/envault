package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const execTestHelperEnv = "ENVAULT_TEST_EXEC_HELPER"

func TestExecHelperProcess(*testing.T) {
	if os.Getenv(execTestHelperEnv) != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	os.Exit(execTestHelperRun(args))
}

func execTestHelperRun(args []string) int {
	if len(args) == 0 {
		return 99
	}
	switch args[0] {
	case "print":
		values := make([]string, 0, len(args)-1)
		for _, key := range args[1:] {
			value, ok := os.LookupEnv(key)
			if !ok {
				value = "<unset>"
			}
			values = append(values, value)
		}
		fmt.Print(strings.Join(values, "|"))
		return 0
	case "exit":
		code, err := strconv.Atoi(args[1])
		if err != nil {
			return 98
		}
		fmt.Fprint(os.Stderr, "saindo")
		return code
	case "stdin":
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			return 97
		}
		return 0
	}
	return 99
}

func execTestChild(t *testing.T, mode ...string) []string {
	t.Helper()
	t.Setenv(execTestHelperEnv, "1")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	return append([]string{"--", os.Args[0], "-test.run=^TestExecHelperProcess$", "--"}, mode...)
}

func execTestRun(t *testing.T, ta *testApp, flags []string, mode ...string) int {
	t.Helper()
	args := append([]string{"exec"}, flags...)
	args = append(args, execTestChild(t, mode...)...)
	return ta.runWith(newExecCmd(ta.App), args...)
}

func TestExecInjectsVariables(t *testing.T) {
	dir := planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := execTestRun(t, ta, []string{"-e", "a"}, "print", "X", "DATABASE_URL"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := ta.Out.String(); got != "1|"+planTestSecretDB {
		t.Fatalf("stdout = %q", got)
	}
	if entries := planTestEntries(t, dir); len(entries) != 0 {
		t.Fatalf("exec created files: %v", entries)
	}
}

func TestExecLastWinsOverProcessEnv(t *testing.T) {
	planTestWorkdir(t)
	t.Setenv("X", "processo")
	t.Setenv("ENVAULT_TEST_EXEC_KEEP", "mantido")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	for _, flags := range [][]string{{"-e", "a,b"}, {"-e", "a", "-e", "b"}, {"--env", "a,b"}} {
		if code := execTestRun(t, ta, flags, "print", "X", "APP_URL", "ENVAULT_TEST_EXEC_KEEP"); code != ExitOK {
			t.Fatalf("%v: code = %d stderr %s", flags, code, ta.Err.String())
		}
		if got := ta.Out.String(); got != "2|"+planTestSecretApp+"|mantido" {
			t.Fatalf("%v: stdout = %q", flags, got)
		}
	}
}

func TestExecPropagatesExitCode(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	for _, want := range []int{3, 42} {
		if code := execTestRun(t, ta, []string{"-e", "a"}, "exit", strconv.Itoa(want)); code != want {
			t.Fatalf("code = %d, want %d", code, want)
		}
		if ta.Err.String() != "saindo" || ta.Out.Len() != 0 {
			t.Fatalf("stdout %q stderr %q", ta.Out.String(), ta.Err.String())
		}
	}
}

func TestExecPassesStdin(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	ta.In.WriteString("entrada do pai")
	if code := execTestRun(t, ta, []string{"-e", "a"}, "stdin"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if ta.Out.String() != "entrada do pai" {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestExecTemplate(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, templateDefaultPath), "PORT=3000\nX=\nSENTRY_DSN=\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := execTestRun(t, ta, []string{"-e", "a"}, "print", "PORT", "X", "DATABASE_URL"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := ta.Out.String(); got != "3000|1|"+planTestSecretDB {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(ta.Err.String(), "SENTRY_DSN") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if code := execTestRun(t, ta, []string{"-e", "a", "--only-template"}, "print", "DATABASE_URL"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if got := ta.Out.String(); got != "<unset>" {
		t.Fatalf("only-template stdout = %q", got)
	}
	if code := execTestRun(t, ta, []string{"-e", "a", "--no-template"}, "print", "PORT"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if got := ta.Out.String(); got != "<unset>" {
		t.Fatalf("no-template stdout = %q", got)
	}
}

func TestExecErrors(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	cases := []struct {
		name string
		args []string
		code int
	}{
		{name: "without env", args: []string{"exec", "--", os.Args[0]}, code: ExitUsage},
		{name: "without command", args: []string{"exec", "-e", "a"}, code: ExitUsage},
		{name: "unknown env", args: []string{"exec", "-e", "a,nao-existe", "--", os.Args[0]}, code: ExitEnvNotFound},
		{name: "command not found", args: []string{"exec", "-e", "a", "--", filepath.Join(t.TempDir(), "nao-existe")}, code: ExitError},
	}
	for _, tc := range cases {
		if code := ta.runWith(newExecCmd(ta.App), tc.args...); code != tc.code {
			t.Errorf("%s: code = %d, want %d (stderr %s)", tc.name, code, tc.code, ta.Err.String())
		}
	}
}
