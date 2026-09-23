package editortest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	ModeEnv      = "ENVAULT_EDITOR_HELPER_DIR"
	HelperTest   = "TestEditorHelperProcess"
	exitNoStep   = 97
	exitBadState = 98
	blockFor     = 60 * time.Second
	pollInterval = 10 * time.Millisecond
	callsFile    = "calls"
	filePerm     = 0o600
)

type Step struct {
	Content string `json:"content"`
	Keep    bool   `json:"keep"`
	Exit    int    `json:"exit"`
	Block   bool   `json:"block"`
}

type Script struct {
	t   testing.TB
	dir string
}

func Command() string {
	return fmt.Sprintf(`"%s" -test.run=%s --`, os.Args[0], HelperTest)
}

func Install(t testing.TB, steps ...Step) *Script {
	t.Helper()
	dir := t.TempDir()
	for i, step := range steps {
		data, err := json.Marshal(step)
		if err != nil {
			t.Fatalf("marshal step %d: %v", i+1, err)
		}
		if err := os.WriteFile(stepPath(dir, "step", i+1), data, filePerm); err != nil {
			t.Fatalf("write step %d: %v", i+1, err)
		}
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", Command())
	t.Setenv(ModeEnv, dir)
	return &Script{t: t, dir: dir}
}

func (s *Script) Calls() int {
	return readCalls(s.dir)
}

func (s *Script) Seen(n int) string {
	return s.read("seen", n)
}

func (s *Script) Path(n int) string {
	return s.read("path", n)
}

func (s *Script) Mode(n int) os.FileMode {
	s.t.Helper()
	mode, err := strconv.ParseUint(s.read("mode", n), 8, 32)
	if err != nil {
		s.t.Fatalf("mode %d: %v", n, err)
	}
	return os.FileMode(mode)
}

func (s *Script) WaitStarted(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stepPath(s.dir, "started", n)); err == nil {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

func (s *Script) read(kind string, n int) string {
	s.t.Helper()
	data, err := os.ReadFile(stepPath(s.dir, kind, n))
	if err != nil {
		s.t.Fatalf("%s %d: %v", kind, n, err)
	}
	return string(data)
}

func Serve() {
	dir := os.Getenv(ModeEnv)
	if dir == "" {
		return
	}
	os.Exit(serve(filepath.Clean(dir), targetArg(os.Args)))
}

func serve(dir, target string) int {
	if target == "" {
		fmt.Fprintln(os.Stderr, "editor falso: arquivo ausente")
		return exitBadState
	}
	target = filepath.Clean(target)
	n := readCalls(dir) + 1
	if err := os.WriteFile(filepath.Join(dir, callsFile), []byte(strconv.Itoa(n)), filePerm); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitBadState
	}
	if err := record(dir, n, target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitBadState
	}
	data, err := os.ReadFile(stepPath(dir, "step", n))
	if err != nil {
		fmt.Fprintf(os.Stderr, "editor falso: passo %d não definido\n", n)
		return exitNoStep
	}
	var step Step
	if err := json.Unmarshal(data, &step); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitBadState
	}
	return apply(dir, n, target, step)
}

func apply(dir string, n int, target string, step Step) int {
	if step.Block {
		if err := os.WriteFile(stepPath(dir, "started", n), nil, filePerm); err != nil {
			return exitBadState
		}
		time.Sleep(blockFor)
		return exitBadState
	}
	if !step.Keep {
		if err := os.WriteFile(target, []byte(step.Content), filePerm); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitBadState
		}
	}
	return step.Exit
}

func record(dir string, n int, target string) error {
	seen, err := os.ReadFile(filepath.Clean(target))
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	files := map[string]string{
		"seen": string(seen),
		"path": target,
		"mode": strconv.FormatUint(uint64(info.Mode().Perm()), 8),
	}
	for kind, content := range files {
		if err := os.WriteFile(stepPath(dir, kind, n), []byte(content), filePerm); err != nil {
			return err
		}
	}
	return nil
}

func readCalls(dir string) int {
	data, err := os.ReadFile(filepath.Clean(filepath.Join(dir, callsFile)))
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return n
}

func targetArg(args []string) string {
	for i, arg := range args {
		if arg == "--" && i+1 < len(args) {
			return args[len(args)-1]
		}
	}
	return ""
}

func stepPath(dir, kind string, n int) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%d", kind, n))
}
