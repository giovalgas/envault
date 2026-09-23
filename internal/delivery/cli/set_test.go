package cli

import (
	"context"
	"testing"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestSetByArgument(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.runWith(newSetCmd(ta.App), "set", "a", "TOKEN=valor")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env, err := ta.vault(t).Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value, _ := env.Lookup("TOKEN"); value != "valor" {
		t.Fatalf("TOKEN = %q", value)
	}
}

func TestSetFromStdin(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	ta.In.WriteString("valor")
	code := ta.runWith(newSetCmd(ta.App), "set", "a", "TOKEN")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env, err := ta.vault(t).Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value, _ := env.Lookup("TOKEN"); value != "valor" {
		t.Fatalf("TOKEN = %q", value)
	}
}

func TestSetInvalidKey(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.runWith(newSetCmd(ta.App), "set", "a", "1BAD=valor")
	if code != ExitValidation {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestSetMissingEquals(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a"})
	code := ta.runWith(newSetCmd(ta.App), "set", "a", "TOKEN=1", "BAD")
	if code != ExitUsage {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestSetMissingEnv(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newSetCmd(ta.App), "set", "nope", "TOKEN=1")
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
