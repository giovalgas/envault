package cli

import (
	"context"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func TestUnsetRemovesKeys(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}})
	code := ta.runWith(newUnsetCmd(ta.App), "unset", "a", "A")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env, err := ta.vault(t).Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, ok := env.Lookup("A"); ok {
		t.Fatalf("A ainda presente")
	}
	if _, ok := env.Lookup("B"); !ok {
		t.Fatalf("B foi removida")
	}
}

func TestUnsetMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newUnsetCmd(ta.App), "unset", "nope", "A")
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
