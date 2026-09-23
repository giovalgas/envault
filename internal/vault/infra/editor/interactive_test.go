package editor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor/editortest"
)

func runInteractive(t *testing.T, session domain.EditorSession) error {
	t.Helper()
	proc := session.Process(context.Background())
	proc.SetStdin(strings.NewReader(""))
	proc.SetStdout(&bytes.Buffer{})
	proc.SetStderr(&bytes.Buffer{})
	proc.SetStdin(nil)
	return proc.Run()
}

func TestInteractiveSessionReopensThenApplies(t *testing.T) {
	script := editortest.Install(t,
		editortest.Step{Content: "A=valor-a-secreto\n1BAD=x\n"},
		editortest.Step{Content: "A=valor-a-secreto\nC=valor-c-secreto\n"},
	)
	session, err := NewInteractive(Options{}).Open(existingEditorEnv(), false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if len(session.Warnings()) != 0 {
		t.Fatalf("warnings = %v", session.Warnings())
	}
	_, err = session.Review(runInteractive(t, session))
	if !errors.Is(err, domain.ErrEditReopen) {
		t.Fatalf("first review err = %v, want ErrEditReopen", err)
	}
	result, err := session.Review(runInteractive(t, session))
	if err != nil || !result.Changed {
		t.Fatalf("second review = %+v, %v", result, err)
	}
	if got := strings.Join(result.Diff.Lines(), "|"); got != "+ C|- B|~ descrição ou tags" {
		t.Fatalf("diff = %q", got)
	}
	if !strings.HasPrefix(script.Seen(2), errorHeaderPrefix) {
		t.Fatalf("reopened content = %q", script.Seen(2))
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(script.Path(1))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp dir still exists: %v", err)
	}
}

func TestInteractiveSessionCancelsNewEnv(t *testing.T) {
	editortest.Install(t, editortest.Step{Content: "# só comentário\n"})
	session, err := NewInteractive(Options{}).Open(domain.Env{Name: "nova"}, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if _, err := session.Review(runInteractive(t, session)); !errors.Is(err, domain.ErrEditCanceled) {
		t.Fatalf("err = %v, want ErrEditCanceled", err)
	}
}

func TestInteractiveWarningsStayOffStderr(t *testing.T) {
	var stderr bytes.Buffer
	session, err := NewInteractive(Options{
		Lookup: editorLookup(map[string]string{EnvEditor: "code"}),
		Stderr: &stderr,
	}).Open(domain.Env{Name: "a"}, false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if warnings := session.Warnings(); len(warnings) != 1 || !strings.Contains(warnings[0], waitFlag) {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestInteractiveOpenRejectsInvalidEditor(t *testing.T) {
	_, err := NewInteractive(Options{Lookup: editorLookup(map[string]string{EnvEditor: `"vi`})}).Open(domain.Env{Name: "a"}, false)
	if !errors.Is(err, ErrNoEditor) {
		t.Fatalf("err = %v", err)
	}
}
