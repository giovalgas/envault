package usecase

import (
	"fmt"

	"github.com/giovalgas/envault/internal/shared/dotenv"
	"github.com/giovalgas/envault/internal/vault/domain"
)

type EnvSource struct {
	Name string
	Read func() ([]byte, error)
}

func (s EnvSource) load() (domain.Env, error) {
	data, err := s.Read()
	if err != nil {
		return domain.Env{}, err
	}
	doc, err := dotenv.Parse(data)
	if err != nil {
		return domain.Env{}, fmt.Errorf("%s: %w", s.Name, err)
	}
	return domain.FromDocument(doc), nil
}
