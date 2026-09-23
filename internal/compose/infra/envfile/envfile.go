package envfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/dotenv"
)

type Files struct{}

func New() Files {
	return Files{}
}

func (Files) Exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, fmt.Errorf("inspecionar destino %s: %w", path, err)
}

func (Files) Write(path string, vars []domain.Var) error {
	return writeDocument(path, dotenv.Document{Vars: toDotenv(vars)})
}

func (Files) Merge(path string, vars []domain.Var) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("ler destino %s: %w", path, err)
	}
	current, err := dotenv.Parse(data)
	if err != nil {
		return 0, fmt.Errorf("mesclar com %s: %w", path, err)
	}
	merged := domain.MergeVars(fromDotenv(current.Vars), vars)
	doc := dotenv.Document{Description: current.Description, Tags: current.Tags, Vars: toDotenv(merged)}
	if err := writeDocument(path, doc); err != nil {
		return 0, err
	}
	return len(merged), nil
}

func toDotenv(vars []domain.Var) []dotenv.Var {
	out := make([]dotenv.Var, len(vars))
	for i, v := range vars {
		out[i] = dotenv.Var(v)
	}
	return out
}

func fromDotenv(vars []dotenv.Var) []domain.Var {
	out := make([]domain.Var, len(vars))
	for i, v := range vars {
		out[i] = domain.Var(v)
	}
	return out
}

func writeDocument(path string, doc dotenv.Document) error {
	return writeAtomic(path, dotenv.Format(doc))
}

func writeAtomic(path string, data []byte) error {
	target, err := resolveTarget(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".envault-*")
	if err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	committed = true
	return nil
}

func resolveTarget(path string) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return path, nil
	case err != nil:
		return "", fmt.Errorf("inspecionar destino %s: %w", path, err)
	case info.IsDir():
		return "", fmt.Errorf("%w: %s", usecase.ErrTargetIsDirectory, path)
	case info.Mode()&fs.ModeSymlink == 0:
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolver link %s: %w", path, err)
	}
	return resolved, nil
}
