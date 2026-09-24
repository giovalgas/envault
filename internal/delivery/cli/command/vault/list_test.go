package vault_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestListJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t,
		vault.Env{
			Name: "postgres-local", Description: "Postgres local", Tags: []string{"db", "local"},
			Vars: []vault.Var{{Key: "DATABASE_URL", Value: "supersegredo"}},
		},
		vault.Env{Name: "stripe-test", Tags: []string{"payments"}},
	)
	code := ta.Run("list", "--json")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "supersegredo") || strings.Contains(ta.Err.String(), "supersegredo") {
		t.Fatalf("valor vazou: stdout=%q stderr=%q", ta.Out.String(), ta.Err.String())
	}
	var envelope presenter.ListEnvelope
	if err := json.Unmarshal(ta.Out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout não é JSON: %v\n%s", err, ta.Out.String())
	}
	if envelope.SchemaVersion != presenter.SchemaVersion || len(envelope.Envs) != 2 {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.Envs[0].Name != "postgres-local" || len(envelope.Envs[0].Keys) != 1 || envelope.Envs[0].Keys[0] != "DATABASE_URL" {
		t.Fatalf("primeira env = %+v", envelope.Envs[0])
	}
}

func TestListSearch(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t,
		vault.Env{Name: "postgres-local", Tags: []string{"db"}},
		vault.Env{Name: "stripe-test"},
	)
	code := ta.Run("list", "--search", "DB")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if !strings.Contains(ta.Out.String(), "postgres-local") || strings.Contains(ta.Out.String(), "stripe-test") {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestListTagFilter(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t,
		vault.Env{Name: "postgres-local", Tags: []string{"db"}},
		vault.Env{Name: "stripe-test", Tags: []string{"payments"}},
	)
	code := ta.Run("list", "--tag", "payments")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "postgres-local") || !strings.Contains(ta.Out.String(), "stripe-test") {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestListNotInitialized(t *testing.T) {
	ta := newTestApp(t)
	code := ta.Run("list")
	if code != presenter.ExitNotInitialized {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if _, err := os.Stat(ta.Home); !os.IsNotExist(err) {
		t.Fatalf("list não deveria criar %q, stat err = %v", ta.Home, err)
	}
}

func TestListEmptyIsSilentInJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.Run("list", "--json")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	var envelope presenter.ListEnvelope
	if err := json.Unmarshal(ta.Out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout não é JSON: %v", err)
	}
	if len(envelope.Envs) != 0 {
		t.Fatalf("envs = %+v", envelope.Envs)
	}
}
