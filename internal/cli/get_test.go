package cli

import (
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func TestGetPrintsValue(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "TOKEN", Value: "valor"}}})
	code := ta.runWith(newGetCmd(ta.App), "get", "a", "TOKEN")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if ta.Out.String() != "valor\n" {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestGetMissingKey(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.runWith(newGetCmd(ta.App), "get", "a", "NOPE")
	if code == ExitOK {
		t.Fatalf("esperava falha, code = %d", code)
	}
}

func TestGetMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newGetCmd(ta.App), "get", "nope", "TOKEN")
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
