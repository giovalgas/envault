package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

type FileWriter struct{}

func NewFileWriter() *FileWriter {
	return &FileWriter{}
}

func (w *FileWriter) Write(ctx context.Context, path string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("criar diretório da skill: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário da skill: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("gravar skill: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("fechar skill: %w", err)
	}
	if err := os.Chmod(tmpName, filePerm); err != nil {
		cleanup()
		return fmt.Errorf("ajustar permissão da skill: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("instalar skill em %s: %w", path, err)
	}
	return nil
}
