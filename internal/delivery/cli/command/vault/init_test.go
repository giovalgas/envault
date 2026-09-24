package vault_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	"github.com/giovalgas/envault/internal/shared/config"
)

func TestInitCreatesVault(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.Run("init"); code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	for _, name := range []string{config.VaultFileName, config.KeyFileName} {
		if _, err := os.Stat(filepath.Join(ta.Home, name)); err != nil {
			t.Fatalf("Stat(%s): %v", name, err)
		}
	}
}

func TestInitIsIdempotent(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.Run("init"); code != presenter.ExitOK {
		t.Fatalf("first run: code = %d, stderr = %q", code, ta.Err.String())
	}
	if code := ta.Run("init"); code != presenter.ExitOK {
		t.Fatalf("second run: code = %d, stderr = %q", code, ta.Err.String())
	}
}
