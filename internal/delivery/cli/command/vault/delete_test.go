package vault_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestDeleteWithYes(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.Run("delete", "a", "--yes")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteWithoutTTY(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.Run("delete", "a")
	if code != presenter.ExitUsage {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); err != nil {
		t.Fatalf("env deveria continuar: %v", err)
	}
}

func TestDeleteWrongConfirmation(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	ta.SetTerminal(true, true)
	ta.In.WriteString("b\n")
	code := ta.Run("delete", "a")
	if code != presenter.ExitCanceled {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); err != nil {
		t.Fatalf("env deveria continuar: %v", err)
	}
}

func TestDeleteRightConfirmation(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	ta.SetTerminal(true, true)
	ta.In.WriteString("a\n")
	code := ta.Run("delete", "a")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.Run("delete", "nope", "--yes")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
