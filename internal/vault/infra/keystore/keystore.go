package keystore

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/giovalgas/envault/internal/vault/domain"
)

func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func DecodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("%w: base64 inválido", domain.ErrMalformedKey)
	}
	if len(key) != domain.KeySize {
		return nil, fmt.Errorf("%w: esperado %d bytes, recebido %d", domain.ErrMalformedKey, domain.KeySize, len(key))
	}
	return key, nil
}

type Static struct {
	key []byte
}

func NewStatic(key []byte) (*Static, error) {
	if len(key) != domain.KeySize {
		return nil, fmt.Errorf("%w: esperado %d bytes, recebido %d", domain.ErrMalformedKey, domain.KeySize, len(key))
	}
	return &Static{key: bytes.Clone(key)}, nil
}

func (s *Static) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return bytes.Clone(s.key), nil
}

func (s *Static) Create(context.Context) ([]byte, error) {
	return nil, fmt.Errorf("%w: definida por variável de ambiente", domain.ErrKeyExists)
}

func WithOverride(override string, fallback domain.KeyStore) (domain.KeyStore, error) {
	if strings.TrimSpace(override) == "" {
		return fallback, nil
	}
	key, err := DecodeKey(override)
	if err != nil {
		return nil, fmt.Errorf("ENVAULT_KEY: %w", err)
	}
	return NewStatic(key)
}

type Chain []domain.KeyStore

func (c Chain) Load(ctx context.Context) ([]byte, error) {
	for _, ks := range c {
		key, err := ks.Load(ctx)
		if errors.Is(err, domain.ErrNoKey) {
			continue
		}
		return key, err
	}
	return nil, domain.ErrNoKey
}

func (c Chain) Create(ctx context.Context) ([]byte, error) {
	if len(c) == 0 {
		return nil, errors.New("nenhum keystore configurado")
	}
	return c[0].Create(ctx)
}

func Account(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	return abs
}
