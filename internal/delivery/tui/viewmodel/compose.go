package viewmodel

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
)

const templateDefaultLabel = "padrão do template"

type PreviewRow struct {
	Key      string
	From     string
	Conflict string
	Shadows  []string
}

func (r PreviewRow) Conflicted() bool {
	return r.Conflict != ""
}

func PreviewRows(plan composeusecase.PlanView, expanded map[string]bool) []PreviewRow {
	rows := make([]PreviewRow, len(plan.Vars))
	for i, resolved := range plan.Vars {
		row := PreviewRow{Key: resolved.Key, From: resolved.From}
		if resolved.Default {
			row.From = templateDefaultLabel
		}
		if n := len(resolved.Shadows); n > 0 {
			marker := "+"
			if expanded[resolved.Key] {
				marker = "-"
			}
			row.Conflict = fmt.Sprintf("conflito %s%d", marker, n)
		}
		if expanded[resolved.Key] {
			for _, shadow := range resolved.Shadows {
				row.Shadows = append(row.Shadows, "sombreada em "+shadow)
			}
		}
		rows[i] = row
	}
	return rows
}

type ChecklistItem struct {
	Text    string
	Missing bool
}

type Checklist struct {
	Header string
	Items  []ChecklistItem
	Extra  string
}

func TemplateChecklist(tmpl composeusecase.TemplateView, plan composeusecase.PlanView) (Checklist, bool) {
	if !tmpl.Found {
		return Checklist{}, false
	}
	missing := plan.Missing
	keys := tmpl.Keys()
	filled := len(keys) - len(missing)
	checklist := Checklist{Header: fmt.Sprintf("template %s: %d de %d preenchidas", tmpl.Path, filled, len(keys))}
	defaults := make(map[string]bool, len(plan.Vars))
	for _, resolved := range plan.Vars {
		defaults[resolved.Key] = resolved.Default
	}
	for _, k := range keys {
		switch {
		case slices.Contains(missing, k):
			checklist.Items = append(checklist.Items, ChecklistItem{Text: MarkOff + " " + k + " faltando", Missing: true})
		case defaults[k]:
			checklist.Items = append(checklist.Items, ChecklistItem{Text: MarkOn + " " + k + " (padrão)"})
		default:
			checklist.Items = append(checklist.Items, ChecklistItem{Text: MarkOn + " " + k})
		}
	}
	if len(plan.Extra) > 0 {
		checklist.Extra = "fora do template: " + strings.Join(plan.Extra, ", ")
	}
	return checklist, true
}

func ExportedText(result composeusecase.LoadShellExportsResult) string {
	text := fmt.Sprintf("%d variáveis de %s exportadas no terminal", result.Written, strings.Join(result.Plan.Envs, ", "))
	if missing := result.Plan.Missing; len(missing) > 0 {
		text += "; sem valor no template: " + strings.Join(missing, ", ")
	}
	return text
}

func CopiedText(result composeusecase.RenderEnvFileResult) string {
	return fmt.Sprintf(".env com %d variáveis de %s copiado para o clipboard", len(result.Plan.Vars), strings.Join(result.Plan.Envs, ", "))
}

func CopiedWarning(result composeusecase.RenderEnvFileResult) string {
	if missing := result.Plan.Missing; len(missing) > 0 {
		return "sem valor no template: " + strings.Join(missing, ", ")
	}
	return ""
}

func WrittenText(target string, result composeusecase.LoadEnvFileResult) string {
	verb := "gravado"
	switch result.Mode {
	case composeusecase.LoadOverwritten:
		verb = "sobrescrito"
	case composeusecase.LoadMerged:
		verb = "mesclado"
	}
	return fmt.Sprintf("%s %s com %d variáveis de %s", target, verb, result.Written, strings.Join(result.Plan.Envs, ", "))
}

func WrittenWarning(target string, result composeusecase.LoadEnvFileResult) string {
	var warnings []string
	if result.Target.Gitignored == composeusecase.GitignoreNotIgnored {
		warnings = append(warnings, fmt.Sprintf("aviso: %s não está coberto pelo .gitignore", target))
	}
	if missing := result.Plan.Missing; len(missing) > 0 {
		warnings = append(warnings, "sem valor no template: "+strings.Join(missing, ", "))
	}
	return strings.Join(warnings, "; ")
}

func TemplateError(err error) error {
	var readErr *composeusecase.TemplateReadError
	var parseErr *composeusecase.TemplateParseError
	switch {
	case errors.As(err, &readErr):
		return fmt.Errorf("ler %s: %w", readErr.Path, readErr.Err)
	case errors.As(err, &parseErr):
		return fmt.Errorf("%s: %w", parseErr.Path, parseErr.Err)
	}
	return err
}
