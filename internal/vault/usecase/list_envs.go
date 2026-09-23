package usecase

import (
	"context"
	"strings"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type ListEnvsQuery struct {
	Search string
	Tag    string
}

type ListEnvs struct {
	repo domain.EnvRepository
}

func NewListEnvs(repo domain.EnvRepository) *ListEnvs {
	return &ListEnvs{repo: repo}
}

func (uc *ListEnvs) Execute(ctx context.Context, query ListEnvsQuery) ([]domain.Env, error) {
	snapshot, err := uc.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	envs := snapshot.Sorted()
	out := make([]domain.Env, 0, len(envs))
	for _, env := range envs {
		if query.matches(env) {
			out = append(out, env)
		}
	}
	return out, nil
}

func (q ListEnvsQuery) matches(env domain.Env) bool {
	if q.Tag != "" && !env.HasTag(q.Tag) {
		return false
	}
	if q.Search == "" {
		return true
	}
	needle := strings.ToLower(q.Search)
	contains := func(s string) bool { return strings.Contains(strings.ToLower(s), needle) }
	if contains(env.Name) || contains(env.Description) {
		return true
	}
	for _, tag := range env.Tags {
		if contains(tag) {
			return true
		}
	}
	for _, key := range env.Keys() {
		if contains(key) {
			return true
		}
	}
	return false
}
