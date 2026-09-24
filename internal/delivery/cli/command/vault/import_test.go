package vault_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestImportCreates(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	path := newWriteFile(t, "# @description: do arquivo\n# @tags: db\nTOKEN=valor-importado\n")
	if code := ta.Run("import", "a", path); code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env := newGetEnv(t, ta)
	if v, _ := env.Lookup("TOKEN"); v != "valor-importado" || env.Description != "do arquivo" || !env.HasTag("db") {
		t.Fatalf("env = %+v", env)
	}
	if !strings.Contains(ta.Err.String(), "criada") || strings.Contains(ta.Err.String(), "valor-importado") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if ta.Out.Len() != 0 {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestImportReplacesExactly(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "OLD", Value: "1"}, {Key: "KEEP", Value: "2"}}})
	path := newWriteFile(t, "KEEP=3\nNEW=4\n")
	if code := ta.Run("import", "a", path); code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env := newGetEnv(t, ta)
	want := []vault.Var{{Key: "KEEP", Value: "3"}, {Key: "NEW", Value: "4"}}
	if !slices.Equal(env.Vars, want) {
		t.Fatalf("vars = %+v", env.Vars)
	}
	if !strings.Contains(ta.Err.String(), "substituída") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestImportDescriptionFlag(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	path := newWriteFile(t, "# @description: do arquivo\nA=1\n")
	if code := ta.Run("import", "a", path, "--description", "da flag"); code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if env := newGetEnv(t, ta); env.Description != "da flag" {
		t.Fatalf("description = %q", env.Description)
	}
}

func TestImportSyntaxErrorKeepsEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "OLD", Value: "1"}}})
	path := newWriteFile(t, "OK=1\nsem igual\n")
	code := ta.Run("import", "a", path)
	if code != presenter.ExitValidation || !strings.Contains(ta.Err.String(), "linha 2") {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if env := newGetEnv(t, ta); !slices.Equal(env.Keys(), []string{"OLD"}) {
		t.Fatalf("keys = %q", env.Keys())
	}
}

func TestImportErrors(t *testing.T) {
	cases := []struct {
		name string
		args func(t *testing.T) []string
		want int
	}{
		{"arquivo ausente", func(t *testing.T) []string {
			return []string{"import", "a", filepath.Join(t.TempDir(), "nao-existe.env")}
		}, presenter.ExitError},
		{"nome inválido", func(t *testing.T) []string {
			return []string{"import", "A B", newWriteFile(t, "A=1\n")}
		}, presenter.ExitValidation},
		{"argumentos faltando", func(*testing.T) []string { return []string{"import", "a"} }, presenter.ExitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ta := newTestApp(t)
			ta.initVault(t)
			if code := ta.Run(tc.args(t)...); code != tc.want {
				t.Fatalf("code = %d, want %d, stderr = %q", code, tc.want, ta.Err.String())
			}
		})
	}
}

func TestImportNotInitialized(t *testing.T) {
	ta := newTestApp(t)
	path := newWriteFile(t, "A=1\n")
	if code := ta.Run("import", "a", path); code != presenter.ExitNotInitialized {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
