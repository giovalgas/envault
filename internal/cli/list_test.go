package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
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
	code := ta.runWith(newListCmd(ta.App), "list", "--json")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "supersegredo") || strings.Contains(ta.Err.String(), "supersegredo") {
		t.Fatalf("valor vazou: stdout=%q stderr=%q", ta.Out.String(), ta.Err.String())
	}
	var envelope listEnvelope
	if err := json.Unmarshal(ta.Out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout não é JSON: %v\n%s", err, ta.Out.String())
	}
	if envelope.SchemaVersion != SchemaVersion || len(envelope.Envs) != 2 {
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
	code := ta.runWith(newListCmd(ta.App), "list", "--search", "DB")
	if code != ExitOK {
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
	code := ta.runWith(newListCmd(ta.App), "list", "--tag", "payments")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "postgres-local") || !strings.Contains(ta.Out.String(), "stripe-test") {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
}

func TestListNotInitialized(t *testing.T) {
	ta := newTestApp(t)
	code := ta.runWith(newListCmd(ta.App), "list")
	if code != ExitNotInitialized {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}

func TestListEmptyIsSilentInJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.runWith(newListCmd(ta.App), "list", "--json")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	var envelope listEnvelope
	if err := json.Unmarshal(ta.Out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout não é JSON: %v", err)
	}
	if len(envelope.Envs) != 0 {
		t.Fatalf("envs = %+v", envelope.Envs)
	}
}
