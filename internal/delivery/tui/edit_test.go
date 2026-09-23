package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/editor/editortest"
)

const (
	secretEdited   = "valor-editado"
	secretImported = "valor-importado"
)

func TestEditorHelperProcess(*testing.T) {
	editortest.Serve()
}

var editSecrets = append([]string{secretEdited, secretImported}, composeSecrets...)

func TestTUIEditAppliesAfterDiff(t *testing.T) {
	script := editortest.Install(t, editortest.Step{Content: "X=" + secretEdited + "\nNOVA=" + secretImported + "\n"})
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("e")
	sess.waitForAll("Gravar alterações em a", "+ NOVA", "~ X")
	if env := s.get(t, "a"); len(env.Vars) != 1 {
		t.Fatalf("env gravada antes da confirmação: %v", env.Keys())
	}
	sess.press(tea.KeyEnter)
	sess.waitFor("a atualizada com 2 chaves")
	m, out := sess.finish()

	if script.Calls() != 1 {
		t.Fatalf("editor chamado %d vezes", script.Calls())
	}
	if !strings.HasSuffix(script.Path(1), ".env") {
		t.Fatalf("temporário = %q", script.Path(1))
	}
	env := s.get(t, "a")
	if value, _ := env.Lookup("X"); value != secretEdited || strings.Join(env.Keys(), ",") != "X,NOVA" {
		t.Fatalf("env = %v", env.Keys())
	}
	if m.statusErr || m.modal != nil {
		t.Fatalf("status = %q modal = %v", m.status, m.modal)
	}
	assertNoSecrets(t, "saída da edição", out, editSecrets...)
}

func TestTUIEditReopensOnParseError(t *testing.T) {
	script := editortest.Install(t,
		editortest.Step{Content: "X=" + secretEdited + "\n1BAD=x\n"},
		editortest.Step{Content: "X=" + secretEdited + "\n"},
	)
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("e")
	sess.waitFor("Gravar alterações em a")
	sess.press(tea.KeyEnter)
	sess.waitFor("a atualizada")
	_, out := sess.finish()

	if script.Calls() != 2 || !strings.HasPrefix(script.Seen(2), "# ERRO linha 2") {
		t.Fatalf("calls = %d, reaberto com %q", script.Calls(), script.Seen(2))
	}
	if value, _ := s.get(t, "a").Lookup("X"); value != secretEdited {
		t.Fatal("edição não gravada")
	}
	assertNoSecrets(t, "saída da edição", out, editSecrets...)
}

func TestTUIEditDiscardKeepsVault(t *testing.T) {
	editortest.Install(t, editortest.Step{Content: "X=" + secretEdited + "\n"})
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("e")
	sess.waitFor("Gravar alterações em a")
	sess.press(tea.KeyTab)
	sess.press(tea.KeyEnter)
	sess.waitFor("alterações em a descartadas")
	sess.finish()
	if value, _ := s.get(t, "a").Lookup("X"); value != secretXFromA {
		t.Fatal("descartar não deveria gravar")
	}
}

func TestTUIEditUnchanged(t *testing.T) {
	editortest.Install(t, editortest.Step{Keep: true})
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("e")
	sess.waitFor("nada mudou em a")
	m, _ := sess.finish()
	if m.modal != nil || m.statusErr {
		t.Fatalf("modal = %v status = %q", m.modal, m.status)
	}
}

func TestTUIEditorFailureIsShown(t *testing.T) {
	editortest.Install(t, editortest.Step{Keep: true, Exit: 3})
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("e")
	sess.waitFor("editor de")
	m, _ := sess.finish()
	if !m.statusErr {
		t.Fatalf("status = %q", m.status)
	}
}

func TestTUIEditCreatesNewEnv(t *testing.T) {
	script := editortest.Install(t, editortest.Step{Content: "# @description: nova env\n# @tags: db\nK=" + secretEdited + "\n"})
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("n")
	sess.waitFor("Nova env")
	sess.typeText("nova")
	sess.press(tea.KeyEnter)
	sess.waitForAll("Criar nova", "+ K")
	sess.press(tea.KeyEnter)
	sess.waitFor("nova criada com 1 chave")
	m, out := sess.finish()

	env := s.get(t, "nova")
	if env.Description != "nova env" || !env.HasTag("db") {
		t.Fatalf("env = %+v", env.Keys())
	}
	if focused, ok := m.list.focused(); !ok || focused.Name != "nova" {
		t.Fatalf("foco = %v", focused.Name)
	}
	if filepath.Base(script.Path(1)) != "nova.env" {
		t.Fatalf("temporário = %q", script.Path(1))
	}
	assertNoSecrets(t, "saída da criação", out, editSecrets...)
}

func TestTUIEditNewEnvCanceled(t *testing.T) {
	editortest.Install(t, editortest.Step{Content: "# só comentário\n"})
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("n")
	sess.waitFor("Nova env")
	sess.typeText("nova")
	sess.press(tea.KeyEnter)
	sess.waitFor("criação de nova cancelada")
	sess.finish()
	if _, err := s.store.Get(context.Background(), "nova"); err == nil {
		t.Fatal("env cancelada foi criada")
	}
}

func TestTUIEditNewEnvRejectsExistingName(t *testing.T) {
	script := editortest.Install(t)
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("n")
	sess.waitFor("Nova env")
	sess.typeText("a")
	sess.press(tea.KeyEnter)
	sess.waitFor(vault.ErrEnvExists.Error())
	m, _ := sess.finish()
	if m.modal == nil || script.Calls() != 0 {
		t.Fatalf("modal = %v calls = %d", m.modal, script.Calls())
	}
}

func TestTUIImportCreatesAndReplaces(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	writeFile(t, filepath.Join(s.dir, defaultEnvFile), "IMPORTADA="+secretImported+"\nOUTRA=1\n")
	sess := s.start(t)
	sess.typeText("i")
	sess.waitFor("Importar .env")
	sess.press(tea.KeyEnter)
	sess.waitFor("Nome da env")
	sess.typeText("c")
	sess.press(tea.KeyEnter)
	sess.waitFor("c criada com 2 chaves de .env")

	sess.typeText("i")
	sess.waitFor("Importar .env")
	sess.press(tea.KeyEnter)
	sess.waitFor("Nome da env")
	sess.typeText("a")
	sess.press(tea.KeyEnter)
	sess.waitFor("Substituir a")
	sess.press(tea.KeyEnter)
	sess.waitFor("a substituída")
	_, out := sess.finish()

	for _, name := range []string{"a", "c"} {
		if keys := strings.Join(s.get(t, name).Keys(), ","); keys != "IMPORTADA,OUTRA" {
			t.Fatalf("%s = %v", name, keys)
		}
	}
	assertNoSecrets(t, "saída da importação", out, editSecrets...)
}

func TestTUIImportMissingFile(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := s.start(t)
	sess.typeText("i")
	sess.waitFor("Importar .env")
	sess.press(tea.KeyEnter)
	sess.waitFor("Nome da env")
	sess.typeText("c")
	sess.press(tea.KeyEnter)
	sess.waitFor("importar .env")
	m, _ := sess.finish()
	if !m.statusErr {
		t.Fatalf("status = %q", m.status)
	}
}

func TestActionsOnlyRegistersAvailableUseCases(t *testing.T) {
	if got := Actions(Deps{}); len(got) != 0 {
		t.Fatalf("Actions vazio = %v", got)
	}
	s := newStack(t)
	got := Actions(s.deps)
	for _, action := range []Action{ActionNew, ActionEdit, ActionImport, ActionCompose} {
		if got[action] == nil {
			t.Errorf("ação %s não registrada", action)
		}
	}
}
