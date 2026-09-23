package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
)

const selectionSecret = "supersegredo"

func selectionSeed() []vault.Env {
	return []vault.Env{
		{Name: "a", Vars: []vault.Var{{Key: "SECRET", Value: selectionSecret}}},
		{Name: "b", Vars: []vault.Var{{Key: "TOKEN", Value: selectionSecret + "-b"}}},
	}
}

type tuiSelecting struct {
	names []string
	err   error
}

func (f *tuiSelecting) run(ctx context.Context, session TUISession) error {
	_, f.err = session.Compose.SaveSelection.Execute(ctx, f.names)
	return f.err
}

func (ta *testApp) saveSelection(t *testing.T, names ...string) {
	t.Helper()
	if _, err := ta.App.Compose().SaveSelection.Execute(context.Background(), names); err != nil {
		t.Fatalf("SaveSelection: %v", err)
	}
}

func (ta *testApp) selectionJSON(t *testing.T) (selectionEnvelope, map[string]json.RawMessage) {
	t.Helper()
	var envelope selectionEnvelope
	if err := json.Unmarshal(ta.Out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout não é JSON: %v\n%s", err, ta.Out.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(ta.Out.Bytes(), &raw); err != nil {
		t.Fatalf("stdout não é JSON: %v", err)
	}
	return envelope, raw
}

func TestSelectionMadeByTUI(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, selectionSeed()...)
	fake := &tuiSelecting{names: []string{"b", "a"}}
	ta.App.Wire.TUI = fake.run
	ta.setTerminal(true, true)
	if code := ta.run(); code != ExitOK || fake.err != nil {
		t.Fatalf("TUI: code = %d err = %v stderr = %q", code, fake.err, ta.Err.String())
	}
	ta.setTerminal(false, false)

	if code := ta.run("selection", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	envelope, raw := ta.selectionJSON(t)
	if envelope.SchemaVersion != SchemaVersion || !slices.Equal(envelope.Envs, []string{"b", "a"}) || len(envelope.Missing) != 0 {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.UpdatedAt == nil || envelope.UpdatedAt.IsZero() {
		t.Fatalf("updated_at = %v", envelope.UpdatedAt)
	}
	if !strings.Contains(ta.Out.String(), `"envs":["b","a"]`) {
		t.Fatalf("stdout = %s", ta.Out.String())
	}
	fields := make([]string, 0, len(raw))
	for field := range raw {
		fields = append(fields, field)
	}
	slices.Sort(fields)
	if !slices.Equal(fields, []string{"envs", "missing", "schema_version", "updated_at"}) {
		t.Fatalf("campos = %v", fields)
	}
}

func TestSelectionReportsMissing(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, selectionSeed()...)
	ta.saveSelection(t, "b", "sumiu", "a")
	if code := ta.run("selection", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	envelope, _ := ta.selectionJSON(t)
	if !slices.Equal(envelope.Envs, []string{"b", "a"}) || !slices.Equal(envelope.Missing, []string{"sumiu"}) {
		t.Fatalf("envelope = %+v", envelope)
	}

	if code := ta.run("selection"); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	if ta.Out.String() != "b\na\n" {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
	if !strings.Contains(ta.Err.String(), "sumiu") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestSelectionAfterCLIRename(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, selectionSeed()...)
	ta.saveSelection(t, "a", "b")
	if code := ta.run("rename", "a", "c"); code != ExitOK {
		t.Fatalf("rename: code = %d stderr = %q", code, ta.Err.String())
	}
	if code := ta.run("selection", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	envelope, _ := ta.selectionJSON(t)
	if !slices.Equal(envelope.Envs, []string{"b"}) || !slices.Equal(envelope.Missing, []string{"a"}) {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestSelectionWithoutSavedSelection(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, selectionSeed()...)
	if code := ta.run("selection", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	envelope, raw := ta.selectionJSON(t)
	if envelope.Envs == nil || len(envelope.Envs) != 0 || envelope.Missing == nil || len(envelope.Missing) != 0 {
		t.Fatalf("envelope = %+v", envelope)
	}
	if string(raw["envs"]) != "[]" || string(raw["missing"]) != "[]" || string(raw["updated_at"]) != "null" {
		t.Fatalf("stdout = %s", ta.Out.String())
	}
	if _, err := os.Stat(filepath.Join(ta.Home, config.SelectionFileName)); !os.IsNotExist(err) {
		t.Fatalf("selection não deveria criar %s: %v", config.SelectionFileName, err)
	}

	if code := ta.run("selection"); code != ExitOK {
		t.Fatalf("code = %d stderr = %q", code, ta.Err.String())
	}
	if ta.Out.Len() != 0 || !strings.Contains(ta.Err.String(), "nenhuma env selecionada") {
		t.Fatalf("stdout = %q stderr = %q", ta.Out.String(), ta.Err.String())
	}
}

func TestSelectionNotInitialized(t *testing.T) {
	for _, args := range [][]string{{"selection"}, {"selection", "--json"}} {
		ta := newTestApp(t)
		if code := ta.run(args...); code != ExitNotInitialized {
			t.Fatalf("%v: code = %d stderr = %q", args, code, ta.Err.String())
		}
		if len(args) == 2 {
			if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != ExitNotInitialized {
				t.Fatalf("envelope = %+v", envelope)
			}
		}
		if _, err := os.Stat(ta.Home); !os.IsNotExist(err) {
			t.Fatalf("selection não deveria criar %q: %v", ta.Home, err)
		}
	}
}

func TestSelectionNeverPrintsValues(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, selectionSeed()...)
	ta.saveSelection(t, "a", "b", "sumiu")
	data, err := os.ReadFile(filepath.Clean(filepath.Join(ta.Home, config.SelectionFileName)))
	if err != nil {
		t.Fatalf("ler selection.json: %v", err)
	}
	if strings.Contains(string(data), selectionSecret) || strings.Contains(string(data), "SECRET") || strings.Contains(string(data), "TOKEN") {
		t.Fatalf("selection.json contém valor ou chave: %s", data)
	}
	for _, args := range [][]string{{"selection"}, {"selection", "--json"}} {
		if code := ta.run(args...); code != ExitOK {
			t.Fatalf("%v: code = %d", args, code)
		}
		for name, out := range map[string]string{"stdout": ta.Out.String(), "stderr": ta.Err.String()} {
			if strings.Contains(out, selectionSecret) || strings.Contains(out, "SECRET") || strings.Contains(out, "TOKEN") {
				t.Fatalf("%v: %s contém valor ou chave: %q", args, name, out)
			}
		}
	}
}

func TestSelectionRejectsArgs(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, selectionSeed()...)
	if code := ta.run("selection", "a"); code != ExitUsage {
		t.Fatalf("code = %d", code)
	}
}
