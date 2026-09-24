package vault_test

import (
	"context"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestRenameMovesEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.Run("rename", "a", "b")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "b"); err != nil {
		t.Fatalf("Get(b): %v", err)
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); err == nil {
		t.Fatalf("a ainda existe")
	}
}

func TestRenameMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.Run("rename", "nope", "b")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestRenameTargetExists(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"}, vault.Env{Name: "b"})
	code := ta.Run("rename", "a", "b")
	if code != presenter.ExitValidation {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
