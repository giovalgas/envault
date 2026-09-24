package vault_test

import (
	"context"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestCopyDuplicatesEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "TOKEN", Value: "valor"}}})
	code := ta.Run("copy", "a", "b")
	if code != presenter.ExitOK {
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
	code := ta.Run("copy", "nope", "b")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestCopyTargetExists(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"}, vault.Env{Name: "b"})
	code := ta.Run("copy", "a", "b")
	if code != presenter.ExitValidation {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
