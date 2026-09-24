package usecase

import (
	"fmt"
	"path/filepath"

	"github.com/giovalgas/envault/internal/shared/dotenv"
	"github.com/giovalgas/envault/internal/vault/domain"
)

func readEnvFile(files EnvFileReader, dir, path string) (domain.Env, error) {
	data, err := files.ReadEnvFile(resolveIn(dir, path))
	if err != nil {
		return domain.Env{}, fmt.Errorf("ler %s: %w", path, err)
	}
	doc, err := dotenv.Parse(data)
	if err != nil {
		return domain.Env{}, fmt.Errorf("%s: %w", path, err)
	}
	return domain.FromDocument(doc), nil
}

func resolveIn(dir, path string) string {
	if dir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}
