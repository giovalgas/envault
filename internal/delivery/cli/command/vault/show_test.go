package vault_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestShowJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{
		Name: "a", Description: "desc", Tags: []string{"db"},
		Vars: []vault.Var{{Key: "SECRET", Value: "supersegredo"}},
	})
	code := ta.Run("show", "a", "--json")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "supersegredo") {
		t.Fatalf("valor vazou: %q", ta.Out.String())
	}
	var envelope presenter.ShowEnvelope
	if err := json.Unmarshal(ta.Out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout não é JSON: %v", err)
	}
	if envelope.Name != "a" || len(envelope.Keys) != 1 || envelope.Keys[0] != "SECRET" {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestShowHuman(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{Name: "a", Vars: []vault.Var{{Key: "SECRET", Value: "supersegredo"}}})
	code := ta.Run("show", "a")
	if code != presenter.ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "supersegredo") || strings.Contains(ta.Err.String(), "supersegredo") {
		t.Fatalf("valor vazou: stdout=%q stderr=%q", ta.Out.String(), ta.Err.String())
	}
	if !strings.Contains(ta.Out.String(), "SECRET") {
		t.Fatalf("nome da chave ausente: %q", ta.Out.String())
	}
}

func TestShowNotFound(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	code := ta.Run("show", "nope", "--json")
	if code != presenter.ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
