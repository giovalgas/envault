package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func TestDeleteWithYes(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.runWith(newDeleteCmd(ta.App), "delete", "a", "--yes")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteWithoutTTY(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.runWith(newDeleteCmd(ta.App), "delete", "a")
	if code != ExitUsage {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); err != nil {
		t.Fatalf("env deveria continuar: %v", err)
	}
}

func TestDeleteWrongConfirmation(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	ta.setTerminal(true, true)
	ta.In.WriteString("b\n")
	code := ta.runWith(newDeleteCmd(ta.App), "delete", "a")
	if code != ExitCanceled {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); err != nil {
		t.Fatalf("env deveria continuar: %v", err)
	}
}

func TestDeleteRightConfirmation(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	ta.setTerminal(true, true)
	ta.In.WriteString("a\n")
	code := ta.runWith(newDeleteCmd(ta.App), "delete", "a")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := ta.vault(t).Get(context.Background(), "a"); !errors.Is(err, vault.ErrEnvNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newDeleteCmd(ta.App), "delete", "nope", "--yes")
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
