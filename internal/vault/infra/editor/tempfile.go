package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/giovalgas/envault/internal/shared/config"
	"github.com/giovalgas/envault/internal/vault/domain"
)

const (
	tempDirPattern  = "envault-"
	tempFileExt     = ".env"
	fallbackTmpName = "env"
)

var sharedMemoryDir = "/dev/shm"

func tempBases(runtimeDir, goos string) []string {
	var bases []string
	if runtimeDir != "" {
		bases = append(bases, runtimeDir)
	}
	if goos != goosWindows {
		bases = append(bases, sharedMemoryDir)
	}
	return append(bases, os.TempDir())
}

func makePrivateDir(runtimeDir, goos string) (string, error) {
	var errs []error
	for _, base := range tempBases(runtimeDir, goos) {
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			continue
		}
		dir, err := os.MkdirTemp(base, tempDirPattern)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if goos != goosWindows {
			if err := os.Chmod(dir, config.DirPerm); err != nil {
				_ = os.RemoveAll(dir)
				errs = append(errs, err)
				continue
			}
		}
		return dir, nil
	}
	return "", fmt.Errorf("criar diretório temporário privado: %w", errors.Join(errs...))
}

func tempFileName(env domain.Env) string {
	if domain.ValidateName(env.Name) == nil {
		return env.Name + tempFileExt
	}
	return fallbackTmpName + tempFileExt
}

func writePrivate(path string, data []byte, goos string) error {
	f, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, config.FilePerm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if goos != goosWindows {
		return os.Chmod(path, config.FilePerm)
	}
	return nil
}

func tempPath(dir string, env domain.Env) string {
	return filepath.Join(dir, tempFileName(env))
}
