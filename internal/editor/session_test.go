package editor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func openEditorSession(t *testing.T, opts Options) *Session {
	t.Helper()
	if opts.Lookup == nil {
		opts.Lookup = editorLookup(map[string]string{EnvEditor: "vi"})
	}
	if opts.Stderr == nil {
		opts.Stderr = &bytes.Buffer{}
	}
	s, err := Open(vault.Env{Name: "postgres-local"}, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func assertEditorPrivate(t *testing.T, s *Session) {
	t.Helper()
	if runtime.GOOS == goosWindows {
		return
	}
	file, err := os.Stat(s.Path())
	if err != nil {
		t.Fatalf("Stat file: %v", err)
	}
	dir, err := os.Stat(filepath.Dir(s.Path()))
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if file.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
		t.Fatalf("modes = %o %o", file.Mode().Perm(), dir.Mode().Perm())
	}
}

func TestEditorTempInRuntimeDir(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("XDG_RUNTIME_DIR e permissões Unix não se aplicam no Windows")
	}
	runtimeDir := t.TempDir()
	s := openEditorSession(t, Options{RuntimeDir: runtimeDir})
	if filepath.Dir(filepath.Dir(s.Path())) != runtimeDir {
		t.Fatalf("path %q not under %q", s.Path(), runtimeDir)
	}
	if filepath.Base(s.Path()) != "postgres-local.env" {
		t.Fatalf("file name = %q", filepath.Base(s.Path()))
	}
	assertEditorPrivate(t, s)
}

func TestEditorTempFallsBackToSharedMemory(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("/dev/shm não existe no Windows")
	}
	shm := t.TempDir()
	previous := sharedMemoryDir
	sharedMemoryDir = shm
	t.Cleanup(func() { sharedMemoryDir = previous })
	s := openEditorSession(t, Options{RuntimeDir: filepath.Join(t.TempDir(), "inexistente")})
	if filepath.Dir(filepath.Dir(s.Path())) != shm {
		t.Fatalf("path %q not under %q", s.Path(), shm)
	}
	assertEditorPrivate(t, s)
}

func TestEditorTempFallsBackToOSTempDir(t *testing.T) {
	previous := sharedMemoryDir
	sharedMemoryDir = filepath.Join(t.TempDir(), "sem-shm")
	t.Cleanup(func() { sharedMemoryDir = previous })
	s := openEditorSession(t, Options{})
	base, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	got, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(s.Path())))
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != base || !strings.HasSuffix(s.Path(), tempFileExt) {
		t.Fatalf("path %q not under %q", s.Path(), base)
	}
	assertEditorPrivate(t, s)
}

func TestEditorTempNameForInvalidEnv(t *testing.T) {
	s, err := Open(vault.Env{Name: "../fora"}, Options{
		Lookup: editorLookup(map[string]string{EnvEditor: "vi"}),
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if filepath.Base(s.Path()) != fallbackTmpName+tempFileExt {
		t.Fatalf("path = %q", s.Path())
	}
}

func TestEditorCloseRemovesTemp(t *testing.T) {
	s := openEditorSession(t, Options{})
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(s.Path())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp dir still exists: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestEditorOpenWarnsAboutWait(t *testing.T) {
	var stderr bytes.Buffer
	s := openEditorSession(t, Options{
		Lookup: editorLookup(map[string]string{EnvEditor: "code"}),
		Stderr: &stderr,
	})
	if !strings.Contains(stderr.String(), waitFlag) {
		t.Fatalf("stderr = %q", stderr.String())
	}
	cmd := s.Command(context.Background())
	if len(cmd.Args) != 3 || cmd.Args[1] != waitFlag || cmd.Args[2] != s.Path() {
		t.Fatalf("args = %q", cmd.Args)
	}
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		t.Fatal("Command must leave stdio unset for tea.ExecProcess")
	}
}

func TestEditorOpenRejectsInvalidEditor(t *testing.T) {
	_, err := Open(vault.Env{Name: "a"}, Options{Lookup: editorLookup(map[string]string{EnvEditor: `"vi`})})
	if !errors.Is(err, ErrNoEditor) {
		t.Fatalf("err = %v", err)
	}
}

func TestEditorStripErrorHeaders(t *testing.T) {
	got := stripErrorHeaders("# ERRO linha 1: x\r\n# ERRO linha 2: y\nA=1\n# ERRO no meio\n")
	if got != "A=1\n# ERRO no meio\n" {
		t.Fatalf("got %q", got)
	}
	if stripErrorHeaders("# ERRO só") != "" {
		t.Fatal("header without newline not stripped")
	}
}
