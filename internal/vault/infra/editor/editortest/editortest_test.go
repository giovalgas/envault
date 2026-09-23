package editortest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeTarget(t *testing.T, content string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "env.env")
	if err := os.WriteFile(target, []byte(content), filePerm); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestInstallConfiguresEditor(t *testing.T) {
	script := Install(t, Step{Content: "A=1\n"})
	if os.Getenv("EDITOR") != Command() || os.Getenv(ModeEnv) != script.dir || os.Getenv("VISUAL") != "" {
		t.Fatalf("env = EDITOR %q, %s %q", os.Getenv("EDITOR"), ModeEnv, os.Getenv(ModeEnv))
	}
	if !strings.Contains(Command(), HelperTest) {
		t.Fatalf("Command = %q", Command())
	}
	if script.Calls() != 0 {
		t.Fatalf("calls = %d", script.Calls())
	}
}

func TestServeRewritesAndRecords(t *testing.T) {
	script := Install(t, Step{Content: "B=2\n", Exit: 3}, Step{Keep: true})
	target := writeTarget(t, "A=1\n")
	if code := serve(script.dir, target); code != 3 {
		t.Fatalf("first code = %d", code)
	}
	if code := serve(script.dir, target); code != 0 {
		t.Fatalf("second code = %d", code)
	}
	if script.Calls() != 2 || script.Seen(1) != "A=1\n" || script.Seen(2) != "B=2\n" || script.Path(1) != target {
		t.Fatalf("calls %d seen %q %q path %q", script.Calls(), script.Seen(1), script.Seen(2), script.Path(1))
	}
	if runtime.GOOS != "windows" && script.Mode(1) != filePerm {
		t.Fatalf("mode = %o", script.Mode(1))
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "B=2\n" {
		t.Fatalf("target = %q, %v", data, err)
	}
	if code := serve(script.dir, target); code != exitNoStep {
		t.Fatalf("missing step code = %d", code)
	}
	if script.WaitStarted(1, 20*time.Millisecond) {
		t.Fatal("WaitStarted true for non-blocking step")
	}
}

func TestServeRejectsBadState(t *testing.T) {
	dir := t.TempDir()
	if code := serve(dir, ""); code != exitBadState {
		t.Fatalf("empty target code = %d", code)
	}
	if code := serve(dir, filepath.Join(dir, "nao-existe")); code != exitBadState {
		t.Fatalf("missing target code = %d", code)
	}
	if err := os.WriteFile(stepPath(dir, "step", 2), []byte("{"), filePerm); err != nil {
		t.Fatal(err)
	}
	if code := serve(dir, writeTarget(t, "")); code != exitBadState {
		t.Fatalf("bad json code = %d", code)
	}
	if code := apply(dir, 1, filepath.Join(dir, "sem", "dir"), Step{Content: "x"}); code != exitBadState {
		t.Fatalf("unwritable target code = %d", code)
	}
}

func TestReadCallsAndTargetArg(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, callsFile), []byte("lixo"), filePerm); err != nil {
		t.Fatal(err)
	}
	if readCalls(dir) != 0 {
		t.Fatal("readCalls accepted garbage")
	}
	if got := targetArg([]string{"bin", "-test.run=X", "--", "--wait", "/tmp/a.env"}); got != "/tmp/a.env" {
		t.Fatalf("targetArg = %q", got)
	}
	if got := targetArg([]string{"bin", "--"}); got != "" {
		t.Fatalf("targetArg without file = %q", got)
	}
}

func TestServeWithoutModeReturns(t *testing.T) {
	t.Setenv(ModeEnv, "")
	Serve()
}
