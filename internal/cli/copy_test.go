package cli

import (
	"context"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func TestCopyDuplicatesEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "TOKEN", Value: "valor"}}})
	code := ta.runWith(newCopyCmd(ta.App), "copy", "a", "b")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	original, err := ta.vault(t).Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get(a): %v", err)
	}
	copied, err := ta.vault(t).Get(context.Background(), "b")
	if err != nil {
		t.Fatalf("Get(b): %v", err)
	}
	if value, _ := copied.Lookup("TOKEN"); value != "valor" {
		t.Fatalf("TOKEN = %q", value)
	}
	if original.Name == copied.Name {
		t.Fatalf("nomes iguais")
	}
}

func TestCopyMissingSource(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newCopyCmd(ta.App), "copy", "nope", "b")
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestCopyTargetExists(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"}, vault.Env{Name: "b"})
	code := ta.runWith(newCopyCmd(ta.App), "copy", "a", "b")
	if code != ExitValidation {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
