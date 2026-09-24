package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type EnvReader interface {
	ReadEnvs(ctx context.Context, names []string) ([]domain.Env, error)
}

type TargetFiles interface {
	Exists(path string) (bool, error)
	Write(path string, vars []domain.Var) error
	Merge(path string, vars []domain.Var) (int, error)
}

type GitignoreChecker interface {
	IsIgnored(path string) domain.GitignoreStatus
}

type ExportWriter interface {
	WriteExports(path string, script string) error
}

type EnvCatalog interface {
	EnvNames(ctx context.Context) ([]string, error)
}

type SelectionStore interface {
	Load(ctx context.Context) (domain.Selection, error)
	Save(ctx context.Context, selection domain.Selection) error
}

type TemplateReader interface {
	ReadTemplate(path string) (data []byte, found bool, err error)
}

type Environment interface {
	Environ() []string
}

type ProcessRunner interface {
	Run(spec ProcessSpec) (ProcessStatus, error)
}
