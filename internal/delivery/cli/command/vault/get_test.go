package vault_test

import (
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestGetPrintsValue(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "TOKEN", Value: "valor"}}})
	code := ta.Run("get", "a", "TOKEN")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if ta.Out.String() != "valor\n" {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestGetMissingKey(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.Run("get", "a", "NOPE")
	if code == presenter.ExitOK {
		t.Fatalf("esperava falha, code = %d", code)
	}
}

func TestGetMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.Run("get", "nope", "TOKEN")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
