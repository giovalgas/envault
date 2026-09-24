package usecase

import (
	"context"
	"errors"

	"github.com/giovalgas/envault/internal/skill/domain"
)

type Writer interface {
	Write(ctx context.Context, path string, content []byte) error
}

type HomeDirResolver interface {
	Dir(ctx context.Context) (string, error)
}

type InstallSkill struct {
	writer  Writer
	homeDir HomeDirResolver
}

func NewInstallSkill(writer Writer, homeDir HomeDirResolver) *InstallSkill {
	return &InstallSkill{writer: writer, homeDir: homeDir}
}

func (uc *InstallSkill) Execute(ctx context.Context, dir string) (string, error) {
	if dir == "" {
		resolved, err := uc.homeDir.Dir(ctx)
		if err != nil {
			return "", err
		}
		dir = resolved
	}
	if dir == "" {
		return "", errors.New("diretório de skills vazio")
	}
	path := domain.Path(dir)
	if err := uc.writer.Write(ctx, path, []byte(domain.Content())); err != nil {
		return "", err
	}
	return path, nil
}
