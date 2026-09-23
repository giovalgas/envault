package vaultsource

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/compose/usecase"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

type OpenListEnvs func() (*vaultusecase.ListEnvs, error)

type Source struct {
	open OpenListEnvs
}

func New(open OpenListEnvs) *Source {
	return &Source{open: open}
}

func (s *Source) ReadEnvs(ctx context.Context, names []string) ([]domain.Env, error) {
	list, err := s.open()
	if err != nil {
		return nil, err
	}
	stored, err := list.Execute(ctx, vaultusecase.ListEnvsQuery{})
	if err != nil {
		return nil, err
	}
	byName := make(map[string]int, len(stored))
	for i, env := range stored {
		byName[env.Name] = i
	}
	envs := make([]domain.Env, 0, len(names))
	for _, name := range names {
		i, ok := byName[name]
		if !ok {
			return nil, &usecase.EnvNotFoundError{Name: name}
		}
		vars := make([]domain.Var, len(stored[i].Vars))
		for j, v := range stored[i].Vars {
			vars[j] = domain.Var{Key: v.Key, Value: v.Value}
		}
		envs = append(envs, domain.Env{Name: name, Vars: vars})
	}
	return envs, nil
}
