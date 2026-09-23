package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/editor/editortest"
	"github.com/giovalgas/envault/internal/vault"
)

func TestEditorHelperProcess(*testing.T) {
	editortest.Serve()
}

func newWriteFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vars.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func newGetEnv(t *testing.T, ta *testApp, name string) vault.Env {
	t.Helper()
	env, err := ta.vault(t).Get(context.Background(), name)
	if err != nil {
		t.Fatalf("Get(%s): %v", name, err)
	}
	return env
}

func newAssertAbsent(t *testing.T, ta *testApp, name string) {
	t.Helper()
	if _, err := ta.vault(t).Get(context.Background(), name); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("Get(%s) err = %v, want ErrEnvNotFound", name, err)
	}
}

func TestNewFromFileWithTags(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	script := editortest.Install(t)
	path := newWriteFile(t, "# @tags: outra\nDATABASE_URL=postgres://localhost\nPOOL=10\n")
	code := ta.runWith(newNewCmd(ta.App), "new", "a", "--from-file", path, "--tags", "db,local")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env := newGetEnv(t, ta, "a")
	if !slices.Equal(env.Tags, []string{"db", "local"}) || !slices.Equal(env.Keys(), []string{"DATABASE_URL", "POOL"}) {
		t.Fatalf("env = %+v", env)
	}
	if script.Calls() != 0 {
		t.Fatalf("editor opened %d time(s)", script.Calls())
	}
	if strings.Contains(ta.Err.String()+ta.Out.String(), "postgres://localhost") {
		t.Fatalf("value leaked: out %q err %q", ta.Out.String(), ta.Err.String())
	}
}

func TestNewFromFileKeepsFileMetadata(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	path := newWriteFile(t, "# @description: do arquivo\n# @tags: db\nA=1\n")
	if code := ta.runWith(newNewCmd(ta.App), "new", "a", "--from-file", path); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env := newGetEnv(t, ta, "a")
	if env.Description != "do arquivo" || !slices.Equal(env.Tags, []string{"db"}) {
		t.Fatalf("env = %+v", env)
	}
}

func TestNewFromFileDescriptionFlagWins(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	path := newWriteFile(t, "# @description: do arquivo\nA=1\n")
	code := ta.runWith(newNewCmd(ta.App), "new", "a", "--from-file", path, "--description", "da flag")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if env := newGetEnv(t, ta, "a"); env.Description != "da flag" {
		t.Fatalf("description = %q", env.Description)
	}
}

func TestNewFromFileSyntaxError(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	path := newWriteFile(t, "A=1\n1KEY=x\n")
	code := ta.runWith(newNewCmd(ta.App), "new", "a", "--from-file", path)
	if code != ExitValidation || !strings.Contains(ta.Err.String(), "linha 2") {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	newAssertAbsent(t, ta, "a")
}

func TestNewFromMissingFile(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newNewCmd(ta.App), "new", "a", "--from-file", filepath.Join(t.TempDir(), "nao-existe.env"))
	if code != ExitError {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestNewWithEditor(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	script := editortest.Install(t, editortest.Step{
		Content: "# @description: banco\n# @tags: db\nDATABASE_URL=postgres://segredo\n",
	})
	code := ta.runWith(newNewCmd(ta.App), "new", "a", "--description", "banco", "--tags", "db")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	seen := script.Seen(1)
	if !strings.Contains(seen, "# @description: banco") || !strings.Contains(seen, "# @tags: db") {
		t.Fatalf("editor received %q", seen)
	}
	if !strings.HasSuffix(script.Path(1), "a.env") {
		t.Fatalf("temp path = %q", script.Path(1))
	}
	env := newGetEnv(t, ta, "a")
	if v, _ := env.Lookup("DATABASE_URL"); v != "postgres://segredo" || env.Description != "banco" {
		t.Fatalf("env = %+v", env)
	}
	if strings.Contains(ta.Err.String()+ta.Out.String(), "postgres://segredo") {
		t.Fatalf("value leaked: out %q err %q", ta.Out.String(), ta.Err.String())
	}
}

func TestNewEditorCanceled(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	editortest.Install(t, editortest.Step{Keep: true})
	if code := ta.runWith(newNewCmd(ta.App), "new", "a"); code != ExitCanceled {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	newAssertAbsent(t, ta, "a")
}

func TestNewEditorReopensThenCreates(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	script := editortest.Install(t,
		editortest.Step{Content: "A=1\n\n\n1KEY=x\n"},
		editortest.Step{Content: "A=1\nKEY=x\n"},
	)
	if code := ta.runWith(newNewCmd(ta.App), "new", "a"); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if !strings.HasPrefix(script.Seen(2), "# ERRO linha 4:") {
		t.Fatalf("second opening = %q", script.Seen(2))
	}
	if env := newGetEnv(t, ta, "a"); !slices.Equal(env.Keys(), []string{"A", "KEY"}) {
		t.Fatalf("keys = %q", env.Keys())
	}
}

func TestNewExistingEnvFailsBeforeEditor(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	script := editortest.Install(t)
	if code := ta.runWith(newNewCmd(ta.App), "new", "a"); code != ExitValidation {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if script.Calls() != 0 {
		t.Fatalf("editor opened %d time(s)", script.Calls())
	}
}

func TestNewInvalidInput(t *testing.T) {
	cases := [][]string{
		{"new", "Nome-Invalido"},
		{"new", "a", "--tags", "DB"},
	}
	for _, args := range cases {
		ta := newTestApp(t)
		ta.initVault(t)
		if code := ta.runWith(newNewCmd(ta.App), args...); code != ExitValidation {
			t.Errorf("%v: code = %d, stderr = %q", args, code, ta.Err.String())
		}
	}
}

func TestNewNotInitialized(t *testing.T) {
	ta := newTestApp(t)
	path := newWriteFile(t, "A=1\n")
	if code := ta.runWith(newNewCmd(ta.App), "new", "a", "--from-file", path); code != ExitNotInitialized {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestNewEditorFailure(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	editortest.Install(t, editortest.Step{Content: "A=1\n", Exit: 2})
	if code := ta.runWith(newNewCmd(ta.App), "new", "a"); code != ExitError {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	newAssertAbsent(t, ta, "a")
}
