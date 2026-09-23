package editor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor/editortest"
)

func TestEditorHelperProcess(*testing.T) {
	editortest.Serve()
}

func editorFlowOptions(isNew bool) (Options, *bytes.Buffer) {
	var stderr bytes.Buffer
	return Options{New: isNew, Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &stderr}, &stderr
}

func existingEditorEnv() domain.Env {
	return domain.Env{
		Name:        "postgres-local",
		Description: "banco local",
		Tags:        []string{"db"},
		Vars:        []domain.Var{{Key: "A", Value: "valor-a-secreto"}, {Key: "B", Value: "valor-b-secreto"}},
	}
}

func assertEditorTempGone(t *testing.T, script *editortest.Script, n int) {
	t.Helper()
	path := script.Path(n)
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp dir of %q still exists: %v", path, err)
	}
}

func TestEditorCancelNewWithOnlyComments(t *testing.T) {
	script := editortest.Install(t, editortest.Step{Content: "# @description: nada\n# só comentário\n\n"})
	opts, _ := editorFlowOptions(true)
	_, err := Edit(context.Background(), domain.Env{Name: "nova"}, opts)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	if script.Calls() != 1 {
		t.Fatalf("calls = %d", script.Calls())
	}
	assertEditorTempGone(t, script, 1)
}

func TestEditorUnchangedContent(t *testing.T) {
	script := editortest.Install(t, editortest.Step{Keep: true})
	opts, _ := editorFlowOptions(false)
	initial := existingEditorEnv()
	result, err := Edit(context.Background(), initial, opts)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if result.Changed || !result.Diff.Empty() || !result.Env.SameContent(initial) {
		t.Fatalf("result = %+v", result)
	}
	seen := script.Seen(1)
	if !strings.Contains(seen, "# @description: banco local") || !strings.Contains(seen, "A=valor-a-secreto") {
		t.Fatalf("editor received %q", seen)
	}
	assertEditorTempGone(t, script, 1)
}

func TestEditorReformattedButSameContent(t *testing.T) {
	editortest.Install(t, editortest.Step{Content: "# @tags: db\n# @description: banco local\nA=valor-a-secreto\nB='valor-b-secreto'\n"})
	opts, _ := editorFlowOptions(false)
	result, err := Edit(context.Background(), existingEditorEnv(), opts)
	if err != nil || result.Changed {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestEditorParseErrorReopens(t *testing.T) {
	bad := "# @description: nova\n\nA=1\n1KEY=x\n"
	script := editortest.Install(t,
		editortest.Step{Content: bad},
		editortest.Step{Content: "A=1\nOK=2\n"},
	)
	opts, _ := editorFlowOptions(true)
	result, err := Edit(context.Background(), domain.Env{Name: "nova"}, opts)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if script.Calls() != 2 {
		t.Fatalf("calls = %d", script.Calls())
	}
	reopened := script.Seen(2)
	if !strings.HasPrefix(reopened, "# ERRO linha 4:") || !strings.Contains(reopened, `"1KEY"`) {
		t.Fatalf("reopened content = %q", reopened)
	}
	if !strings.HasSuffix(reopened, bad) || strings.Count(reopened, "# ERRO") != 1 {
		t.Fatalf("user text not preserved: %q", reopened)
	}
	if script.Path(1) != script.Path(2) {
		t.Fatalf("reopened a different file: %q %q", script.Path(1), script.Path(2))
	}
	if !result.Changed || !slices.Equal(result.Env.Keys(), []string{"A", "OK"}) || result.Env.Name != "nova" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEditorRepeatedErrorKeepsSingleHeader(t *testing.T) {
	script := editortest.Install(t,
		editortest.Step{Content: "1KEY=x\n"},
		editortest.Step{Keep: true},
		editortest.Step{Content: ""},
	)
	opts, _ := editorFlowOptions(false)
	_, err := Edit(context.Background(), existingEditorEnv(), opts)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	third := script.Seen(3)
	if strings.Count(third, "# ERRO") != 1 || !strings.HasPrefix(third, "# ERRO linha 2:") {
		t.Fatalf("third opening = %q", third)
	}
	assertEditorTempGone(t, script, 3)
}

func TestEditorGiveUpAfterError(t *testing.T) {
	script := editortest.Install(t,
		editortest.Step{Content: "A=1\nsem igual\n"},
		editortest.Step{Content: ""},
	)
	opts, _ := editorFlowOptions(true)
	_, err := Edit(context.Background(), domain.Env{Name: "nova"}, opts)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	if !strings.HasPrefix(script.Seen(2), "# ERRO linha 2:") {
		t.Fatalf("second opening = %q", script.Seen(2))
	}
}

func TestEditorSuccessWithDiff(t *testing.T) {
	script := editortest.Install(t, editortest.Step{
		Content: "# @description: banco local\n# @tags: db, local\nA=valor-a-secreto\nB=valor-b-novo\nC=valor-c-novo\n",
	})
	opts, _ := editorFlowOptions(false)
	initial := existingEditorEnv()
	result, err := Edit(context.Background(), initial, opts)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	d := result.Diff
	if !result.Changed || !slices.Equal(d.Added, []string{"C"}) || len(d.Removed) != 0 || !slices.Equal(d.Changed, []string{"B"}) || !d.Metadata {
		t.Fatalf("result = %+v", result)
	}
	if v, _ := result.Env.Lookup("C"); v != "valor-c-novo" || result.Env.Name != initial.Name {
		t.Fatalf("env = %+v", result.Env)
	}
	text := d.String()
	for _, v := range append(initial.Vars, result.Env.Vars...) {
		if strings.Contains(text, v.Value) {
			t.Fatalf("diff leaks %q: %q", v.Value, text)
		}
	}
	assertEditorTempGone(t, script, 1)
}

func TestEditorFileIsPrivate(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("permissões Unix não se aplicam no Windows")
	}
	runtimeDir := t.TempDir()
	script := editortest.Install(t, editortest.Step{Keep: true})
	opts, _ := editorFlowOptions(false)
	opts.RuntimeDir = runtimeDir
	if _, err := Edit(context.Background(), existingEditorEnv(), opts); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	path := script.Path(1)
	if !strings.HasSuffix(path, ".env") || !strings.HasPrefix(path, runtimeDir) {
		t.Fatalf("path = %q", path)
	}
	if mode := script.Mode(1); mode != 0o600 {
		t.Fatalf("mode = %o", mode)
	}
}

func TestEditorFailureCleansUp(t *testing.T) {
	script := editortest.Install(t, editortest.Step{Content: "A=1\n", Exit: 3})
	opts, _ := editorFlowOptions(false)
	_, err := Edit(context.Background(), existingEditorEnv(), opts)
	if !errors.Is(err, ErrEditor) {
		t.Fatalf("err = %v, want ErrEditor", err)
	}
	assertEditorTempGone(t, script, 1)
}

func TestEditorMissingBinary(t *testing.T) {
	t.Setenv(EnvVisual, "")
	t.Setenv(EnvEditor, filepath.Join(t.TempDir(), "nao-existe"))
	opts, _ := editorFlowOptions(false)
	if _, err := Edit(context.Background(), existingEditorEnv(), opts); !errors.Is(err, ErrEditor) {
		t.Fatalf("err = %v, want ErrEditor", err)
	}
}

func TestEditorSignalCleansUp(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("SIGTERM não é entregue a processos no Windows")
	}
	script := editortest.Install(t, editortest.Step{Block: true})
	opts, _ := editorFlowOptions(false)
	done := make(chan error, 1)
	go func() {
		_, err := Edit(context.Background(), existingEditorEnv(), opts)
		done <- err
	}()
	if !script.WaitStarted(1, 20*time.Second) {
		t.Fatal("fake editor did not start")
	}
	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if err := self.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrCanceled) {
			t.Fatalf("err = %v, want ErrCanceled", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Edit did not return after SIGTERM")
	}
	assertEditorTempGone(t, script, 1)
}

func TestEditorAdapterPassesMode(t *testing.T) {
	editortest.Install(t, editortest.Step{Content: "# só comentário\n"}, editortest.Step{Keep: true})
	opts, _ := editorFlowOptions(false)
	adapter := New(opts)
	if _, err := adapter.Edit(context.Background(), domain.Env{Name: "nova"}, true); !errors.Is(err, domain.ErrEditCanceled) {
		t.Fatalf("new err = %v, want ErrEditCanceled", err)
	}
	result, err := adapter.Edit(context.Background(), existingEditorEnv(), false)
	if err != nil || result.Changed {
		t.Fatalf("existing = %+v, %v", result, err)
	}
}
