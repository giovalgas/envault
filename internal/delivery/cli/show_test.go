package cli

import (
	"encoding/json"
	"strings"
	"testing"

	vault "github.com/giovalgas/envault/internal/vault/domain"
)

func TestShowJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, vault.Env{
		Name: "a", Description: "desc", Tags: []string{"db"},
		Vars: []vault.Var{{Key: "SECRET", Value: "supersegredo"}},
	})
	code := ta.runWith(newShowCmd(ta.App), "show", "a", "--json")
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if strings.Contains(ta.Out.String(), "supersegredo") {
		t.Fatalf("valor vazou: %q", ta.Out.String())
	}
	var envelope showEnvelope
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
	code := ta.runWith(newShowCmd(ta.App), "show", "a")
	if code != ExitOK {
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
	code := ta.runWith(newShowCmd(ta.App), "show", "nope", "--json")
	if code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
}
