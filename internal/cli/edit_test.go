package cli

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/editor/editortest"
	"github.com/giovalgas/envault/internal/vault"
)

func editSeedEnv() vault.Env {
	return vault.Env{
		Name:        "a",
		Description: "banco",
		Tags:        []string{"db"},
		Vars:        []vault.Var{{Key: "A", Value: "valor-a-secreto"}, {Key: "B", Value: "valor-b-secreto"}},
	}
}

func TestEditUnchanged(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, editSeedEnv())
	editortest.Install(t, editortest.Step{Keep: true})
	if code := ta.runWith(newEditCmd(ta.App), "edit", "a"); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if !strings.Contains(ta.Err.String(), "nada mudou") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if env := newGetEnv(t, ta, "a"); !env.SameContent(editSeedEnv()) {
		t.Fatalf("env changed: %+v", env)
	}
}

func TestEditAppliesAndReportsDiffWithoutValues(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, editSeedEnv())
	editortest.Install(t, editortest.Step{
		Content: "# @description: banco novo\n# @tags: db\nA=valor-a-secreto\nB=valor-b-trocado\nC=valor-c-novo\n",
	})
	if code := ta.runWith(newEditCmd(ta.App), "edit", "a"); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	env := newGetEnv(t, ta, "a")
	if v, _ := env.Lookup("B"); v != "valor-b-trocado" || env.Description != "banco novo" {
		t.Fatalf("env = %+v", env)
	}
	if !slices.Equal(env.Keys(), []string{"A", "B", "C"}) {
		t.Fatalf("keys = %q", env.Keys())
	}
	stderr := ta.Err.String()
	for _, want := range []string{"+ C", "~ B", "descrição ou tags"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr %q misses %q", stderr, want)
		}
	}
	for _, v := range append(editSeedEnv().Vars, env.Vars...) {
		if strings.Contains(stderr, v.Value) || strings.Contains(ta.Out.String(), v.Value) {
			t.Fatalf("value %q leaked: out %q err %q", v.Value, ta.Out.String(), stderr)
		}
	}
}

func TestEditRemovesKey(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, editSeedEnv())
	editortest.Install(t, editortest.Step{Content: "# @description: banco\n# @tags: db\nB=valor-b-secreto\n"})
	if code := ta.runWith(newEditCmd(ta.App), "edit", "a"); code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if !strings.Contains(ta.Err.String(), "- A") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if env := newGetEnv(t, ta, "a"); !slices.Equal(env.Keys(), []string{"B"}) {
		t.Fatalf("keys = %q", env.Keys())
	}
}

func TestEditEmptyFileCancels(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, editSeedEnv())
	editortest.Install(t, editortest.Step{Content: "\n"})
	if code := ta.runWith(newEditCmd(ta.App), "edit", "a"); code != ExitCanceled {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if env := newGetEnv(t, ta, "a"); !env.SameContent(editSeedEnv()) {
		t.Fatalf("env changed: %+v", env)
	}
}

func TestEditParseErrorReopensThenGiveUp(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, editSeedEnv())
	script := editortest.Install(t,
		editortest.Step{Content: "A=1\nB=2\nC=3\n1KEY=x\n"},
		editortest.Step{Content: ""},
	)
	if code := ta.runWith(newEditCmd(ta.App), "edit", "a"); code != ExitCanceled {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if seen := script.Seen(2); !strings.HasPrefix(seen, "# ERRO linha 4:") || !strings.Contains(seen, "1KEY") {
		t.Fatalf("second opening = %q", seen)
	}
	if env := newGetEnv(t, ta, "a"); !env.SameContent(editSeedEnv()) {
		t.Fatalf("env changed: %+v", env)
	}
}

func TestEditNotFound(t *testing.T) {
	ta := newTestApp(t)
	ta.initVault(t)
	script := editortest.Install(t)
	if code := ta.runWith(newEditCmd(ta.App), "edit", "nao-existe"); code != ExitEnvNotFound {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if script.Calls() != 0 {
		t.Fatalf("editor opened %d time(s)", script.Calls())
	}
}

func TestEditEditorFailureKeepsVault(t *testing.T) {
	ta := newTestApp(t)
	ta.seed(t, editSeedEnv())
	editortest.Install(t, editortest.Step{Content: "A=outro\n", Exit: 1})
	if code := ta.runWith(newEditCmd(ta.App), "edit", "a"); code != ExitError {
		t.Fatalf("code = %d, stderr = %q", code, ta.Err.String())
	}
	if env := newGetEnv(t, ta, "a"); !env.SameContent(editSeedEnv()) {
		t.Fatalf("env changed: %+v", env)
	}
}

func TestEditApplyDetectsConcurrentChange(t *testing.T) {
	ta := newTestApp(t)
	v := ta.seed(t, editSeedEnv())
	initial := newGetEnv(t, ta, "a")
	if _, err := v.Modify(context.Background(), "a", func(env *vault.Env) error {
		env.Set("X", "1")
		return nil
	}); err != nil {
		t.Fatalf("Modify: %v", err)
	}
	edited := initial.Clone()
	edited.Vars = []vault.Var{{Key: "Z", Value: "z"}}
	if err := editApply(context.Background(), v, initial, edited); err == nil || !strings.Contains(err.Error(), "mudou") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := newGetEnv(t, ta, "a").Lookup("X"); !ok {
		t.Fatal("concurrent change lost")
	}
}
