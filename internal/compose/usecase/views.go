package usecase

import (
	"path/filepath"
	"time"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type GitignoreStatus = domain.GitignoreStatus

const (
	GitignoreUnknown    = domain.GitignoreUnknown
	GitignoreIgnored    = domain.GitignoreIgnored
	GitignoreNotIgnored = domain.GitignoreNotIgnored
)

type ResolvedView struct {
	Key     string
	Value   string
	From    string
	Shadows []string
	Default bool
}

type PlanView struct {
	Envs      []string
	Vars      []ResolvedView
	Conflicts []string
	Missing   []string
	Extra     []string
}

func (p PlanView) Keys() []string {
	keys := make([]string, len(p.Vars))
	for i, resolved := range p.Vars {
		keys[i] = resolved.Key
	}
	return keys
}

type TargetView struct {
	Path       string
	Exists     bool
	Gitignored GitignoreStatus
}

type SelectionView struct {
	Envs      []string
	UpdatedAt time.Time
}

func (s SelectionView) Saved() bool {
	return !s.UpdatedAt.IsZero()
}

type TemplateEntryView struct {
	Key        string
	Default    string
	HasDefault bool
}

type TemplateView struct {
	Path    string
	Found   bool
	Entries []TemplateEntryView
}

func (t TemplateView) Keys() []string {
	keys := make([]string, len(t.Entries))
	for i, entry := range t.Entries {
		keys[i] = entry.Key
	}
	return keys
}

func planView(plan domain.Plan) PlanView {
	vars := make([]ResolvedView, len(plan.Vars))
	for i, resolved := range plan.Vars {
		vars[i] = ResolvedView{
			Key:     resolved.Key,
			Value:   resolved.Value,
			From:    resolved.From,
			Shadows: resolved.ShadowNames(),
			Default: resolved.Default,
		}
	}
	return PlanView{Envs: plan.Envs, Vars: vars, Conflicts: plan.Conflicts, Missing: plan.Missing, Extra: plan.Extra}
}

func targetView(target domain.Target) TargetView {
	return TargetView{Path: target.Path, Exists: target.Exists, Gitignored: target.Gitignored}
}

func selectionView(selection domain.Selection) SelectionView {
	return SelectionView{Envs: selection.Envs, UpdatedAt: selection.UpdatedAt}
}

func (t TemplateView) toDomain() *domain.Template {
	if !t.Found {
		return nil
	}
	entries := make([]domain.TemplateEntry, len(t.Entries))
	for i, entry := range t.Entries {
		entries[i] = domain.TemplateEntry(entry)
	}
	return &domain.Template{Entries: entries}
}

func resolveIn(dir, path string) string {
	if dir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}
