package usecase

import (
	"slices"
	"time"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type VarView struct {
	Key   string
	Value string
}

type EnvView struct {
	Name        string
	Description string
	Tags        []string
	Vars        []VarView
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e EnvView) Keys() []string {
	keys := make([]string, len(e.Vars))
	for i, v := range e.Vars {
		keys[i] = v.Key
	}
	return keys
}

func (e EnvView) Clone() EnvView {
	out := e
	out.Tags = slices.Clone(e.Tags)
	out.Vars = slices.Clone(e.Vars)
	return out
}

type DiffView struct {
	Added     []string
	Removed   []string
	Changed   []string
	Metadata  bool
	Reordered bool
}

func (d DiffView) Lines() []string {
	return domain.Diff(d).Lines()
}

type EditResultView struct {
	Env     EnvView
	Changed bool
	Diff    DiffView
}

func envView(env domain.Env) EnvView {
	view := EnvView{
		Name:        env.Name,
		Description: env.Description,
		Tags:        slices.Clone(env.Tags),
		CreatedAt:   env.CreatedAt,
		UpdatedAt:   env.UpdatedAt,
	}
	if env.Vars != nil {
		view.Vars = make([]VarView, len(env.Vars))
		for i, v := range env.Vars {
			view.Vars[i] = VarView(v)
		}
	}
	return view
}

func envViews(envs []domain.Env) []EnvView {
	views := make([]EnvView, len(envs))
	for i, env := range envs {
		views[i] = envView(env)
	}
	return views
}

func envFromView(view EnvView) domain.Env {
	return domain.Env{
		Name:        view.Name,
		Description: view.Description,
		Tags:        slices.Clone(view.Tags),
		Vars:        varsFromViews(view.Vars),
		CreatedAt:   view.CreatedAt,
		UpdatedAt:   view.UpdatedAt,
	}
}

func varsFromViews(views []VarView) []domain.Var {
	if views == nil {
		return nil
	}
	vars := make([]domain.Var, len(views))
	for i, v := range views {
		vars[i] = domain.Var(v)
	}
	return vars
}

func editResultView(result domain.EditResult) EditResultView {
	return EditResultView{Env: envView(result.Env), Changed: result.Changed, Diff: DiffView(result.Diff)}
}

func editResultFromView(view EditResultView) domain.EditResult {
	return domain.EditResult{Env: envFromView(view.Env), Changed: view.Changed, Diff: domain.Diff(view.Diff)}
}
