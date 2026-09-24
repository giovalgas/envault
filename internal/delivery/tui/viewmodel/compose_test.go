package viewmodel

import (
	"errors"
	"slices"
	"testing"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
)

func conflictPlan() composeusecase.PlanView {
	return composeusecase.PlanView{
		Envs: []string{"a", "b", "c"},
		Vars: []composeusecase.ResolvedView{
			{Key: "X", Value: "valor-x", From: "c", Shadows: []string{"a", "b"}},
			{Key: "Y", Value: "valor-y", From: "b"},
			{Key: "PORT", Value: "3000", Default: true},
		},
		Conflicts: []string{"X"},
		Missing:   []string{"SENTRY_DSN"},
		Extra:     []string{"X", "Y"},
	}
}

func TestPreviewRowsMarkConflicts(t *testing.T) {
	rows := PreviewRows(conflictPlan(), map[string]bool{})
	if len(rows) != 3 {
		t.Fatalf("linhas = %+v", rows)
	}
	if rows[0].Conflict != "conflito +2" || !rows[0].Conflicted() || len(rows[0].Shadows) != 0 {
		t.Fatalf("conflito recolhido = %+v", rows[0])
	}
	if rows[1].Conflicted() || rows[1].From != "b" {
		t.Fatalf("sem conflito = %+v", rows[1])
	}
	if rows[2].From != "padrão do template" {
		t.Fatalf("padrão = %+v", rows[2])
	}
	expanded := PreviewRows(conflictPlan(), map[string]bool{"X": true})
	if expanded[0].Conflict != "conflito -2" || !slices.Equal(expanded[0].Shadows, []string{"sombreada em a", "sombreada em b"}) {
		t.Fatalf("conflito expandido = %+v", expanded[0])
	}
	for _, row := range expanded {
		for _, text := range append([]string{row.Key, row.From, row.Conflict}, row.Shadows...) {
			if text == "valor-x" || text == "valor-y" || text == "3000" {
				t.Fatalf("prévia expõe valor: %+v", row)
			}
		}
	}
}

func TestTemplateChecklist(t *testing.T) {
	if _, found := TemplateChecklist(composeusecase.TemplateView{}, conflictPlan()); found {
		t.Fatal("sem template não há checklist")
	}
	tmpl := composeusecase.TemplateView{Path: ".env.example", Found: true, Entries: []composeusecase.TemplateEntryView{
		{Key: "Y"}, {Key: "PORT", Default: "3000", HasDefault: true}, {Key: "SENTRY_DSN"},
	}}
	checklist, found := TemplateChecklist(tmpl, conflictPlan())
	if !found || checklist.Header != "template .env.example: 2 de 3 preenchidas" {
		t.Fatalf("checklist = %+v", checklist)
	}
	want := []ChecklistItem{
		{Text: "[x] Y"},
		{Text: "[x] PORT (padrão)"},
		{Text: "[ ] SENTRY_DSN faltando", Missing: true},
	}
	if !slices.Equal(checklist.Items, want) {
		t.Fatalf("itens = %+v", checklist.Items)
	}
	if checklist.Extra != "fora do template: X, Y" {
		t.Fatalf("extra = %q", checklist.Extra)
	}
}

func TestComposeResultTexts(t *testing.T) {
	plan := composeusecase.PlanView{Envs: []string{"a", "b"}}
	exported := ExportedText(composeusecase.LoadShellExportsResult{Written: 2, Plan: plan})
	if exported != "2 variáveis de a, b exportadas no terminal" {
		t.Fatalf("exported = %q", exported)
	}
	planMissing := composeusecase.PlanView{Envs: []string{"a"}, Missing: []string{"SENTRY_DSN"}}
	if got := ExportedText(composeusecase.LoadShellExportsResult{Written: 1, Plan: planMissing}); got != "1 variáveis de a exportadas no terminal; sem valor no template: SENTRY_DSN" {
		t.Fatalf("exported com faltantes = %q", got)
	}
	for mode, verb := range map[composeusecase.LoadMode]string{
		composeusecase.LoadCreated:     "gravado",
		composeusecase.LoadOverwritten: "sobrescrito",
		composeusecase.LoadMerged:      "mesclado",
	} {
		got := WrittenText(".env", composeusecase.LoadEnvFileResult{Mode: mode, Written: 2, Plan: plan})
		if got != ".env "+verb+" com 2 variáveis de a, b" {
			t.Errorf("modo %v: %q", mode, got)
		}
	}
	result := composeusecase.LoadEnvFileResult{Plan: planMissing, Target: composeusecase.TargetView{Gitignored: composeusecase.GitignoreNotIgnored}}
	if got := WrittenWarning("app.env", result); got != "aviso: app.env não está coberto pelo .gitignore; sem valor no template: SENTRY_DSN" {
		t.Fatalf("warning = %q", got)
	}
	if got := WrittenWarning(".env", composeusecase.LoadEnvFileResult{}); got != "" {
		t.Fatalf("warning vazio = %q", got)
	}
}

func TestTemplateError(t *testing.T) {
	cause := errors.New("sem permissão")
	readErr := TemplateError(&composeusecase.TemplateReadError{Path: ".env.example", Err: cause})
	if readErr.Error() != "ler .env.example: sem permissão" || !errors.Is(readErr, cause) {
		t.Fatalf("read = %v", readErr)
	}
	parseErr := TemplateError(&composeusecase.TemplateParseError{Path: ".env.example", Err: cause})
	if parseErr.Error() != ".env.example: sem permissão" || !errors.Is(parseErr, cause) {
		t.Fatalf("parse = %v", parseErr)
	}
	if other := TemplateError(cause); !errors.Is(other, cause) || other.Error() != cause.Error() {
		t.Fatalf("outro = %v", other)
	}
}
