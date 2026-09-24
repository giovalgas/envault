package selectionfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/giovalgas/envault/internal/compose/domain"
	"github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/shared/config"
)

const SchemaVersion = 1

var ErrUnsupportedSchema = errors.New("versão de selection.json não suportada")

type LocateDir func() (string, error)

type Store struct {
	dir LocateDir
}

var _ usecase.SelectionStore = (*Store)(nil)

type document struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Envs          []string  `json:"envs"`
}

func New(dir LocateDir) *Store {
	return &Store{dir: dir}
}

func (s *Store) Load(context.Context) (domain.Selection, error) {
	path, err := s.path()
	if err != nil {
		return domain.Selection{}, err
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return domain.NewSelection(nil, time.Time{})
	}
	if err != nil {
		return domain.Selection{}, fmt.Errorf("ler %s: %w", config.SelectionFileName, err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return domain.Selection{}, fmt.Errorf("ler %s: %w", config.SelectionFileName, err)
	}
	if doc.SchemaVersion != SchemaVersion {
		return domain.Selection{}, fmt.Errorf("%w: %d", ErrUnsupportedSchema, doc.SchemaVersion)
	}
	selection, err := domain.NewSelection(doc.Envs, doc.UpdatedAt)
	if err != nil {
		return domain.Selection{}, fmt.Errorf("ler %s: %w", config.SelectionFileName, err)
	}
	return selection, nil
}

func (s *Store) Save(_ context.Context, selection domain.Selection) error {
	path, err := s.path()
	if err != nil {
		return err
	}
	envs := selection.Envs
	if envs == nil {
		envs = []string{}
	}
	data, err := json.MarshalIndent(document{
		SchemaVersion: SchemaVersion,
		UpdatedAt:     selection.UpdatedAt.UTC(),
		Envs:          envs,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("gravar %s: %w", config.SelectionFileName, err)
	}
	return writeAtomic(path, append(data, '\n'))
}

func (s *Store) path() (string, error) {
	dir, err := s.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, config.SelectionFileName), nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".envault-*")
	if err != nil {
		return fmt.Errorf("gravar %s: %w", config.SelectionFileName, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	steps := []func() error{
		func() error { return tmp.Chmod(config.FilePerm) },
		func() error { _, err := tmp.Write(data); return err },
		tmp.Sync,
		tmp.Close,
		func() error { return os.Rename(tmp.Name(), path) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return fmt.Errorf("gravar %s: %w", config.SelectionFileName, err)
		}
	}
	committed = true
	return nil
}
