package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/giovalgas/envault/internal/config"
	"github.com/giovalgas/envault/internal/crypto"
)

type File struct {
	path string
}

func NewFile(path string) *File {
	return &File{path: filepath.Clean(path)}
}

func (f *File) Path() string {
	return f.path
}

func (f *File) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, fmt.Errorf("ler chave: %w", err)
	}
	key, err := DecodeKey(string(raw))
	if err != nil {
		return nil, fmt.Errorf("arquivo %s: %w", f.path, err)
	}
	return key, nil
}

func (f *File) Create(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := crypto.NewKey()
	if err != nil {
		return nil, err
	}
	if err := f.write(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (f *File) Remove(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoKey
	}
	if err != nil {
		return fmt.Errorf("remover chave: %w", err)
	}
	return nil
}

func (f *File) write(key []byte) error {
	if err := os.MkdirAll(filepath.Dir(f.path), config.DirPerm); err != nil {
		return fmt.Errorf("criar diretório da chave: %w", err)
	}
	file, err := os.OpenFile(f.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, config.FilePerm)
	if errors.Is(err, fs.ErrExist) {
		return ErrKeyExists
	}
	if err != nil {
		return fmt.Errorf("criar arquivo da chave: %w", err)
	}
	if _, err := file.WriteString(EncodeKey(key) + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(f.path)
		return fmt.Errorf("gravar chave: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(f.path)
		return fmt.Errorf("sincronizar chave: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(f.path)
		return fmt.Errorf("fechar chave: %w", err)
	}
	return nil
}
