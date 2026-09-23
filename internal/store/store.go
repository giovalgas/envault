package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/giovalgas/envault/internal/crypto"
)

var (
	ErrNoKey        = errors.New("chave não encontrada")
	ErrKeyExists    = errors.New("chave já existe")
	ErrMalformedKey = errors.New("chave inválida")
)

type KeyStore interface {
	Load(ctx context.Context) ([]byte, error)
	Create(ctx context.Context) ([]byte, error)
}

func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func DecodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("%w: base64 inválido", ErrMalformedKey)
	}
	if len(key) != crypto.KeySize {
		return nil, fmt.Errorf("%w: esperado %d bytes, recebido %d", ErrMalformedKey, crypto.KeySize, len(key))
	}
	return key, nil
}

type Static struct {
	key []byte
}

func NewStatic(key []byte) (*Static, error) {
	if len(key) != crypto.KeySize {
		return nil, fmt.Errorf("%w: esperado %d bytes, recebido %d", ErrMalformedKey, crypto.KeySize, len(key))
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
	return nil, fmt.Errorf("%w: definida por variável de ambiente", ErrKeyExists)
}

func WithOverride(override string, fallback KeyStore) (KeyStore, error) {
	if strings.TrimSpace(override) == "" {
		return fallback, nil
	}
	key, err := DecodeKey(override)
	if err != nil {
		return nil, fmt.Errorf("ENVAULT_KEY: %w", err)
	}
	return NewStatic(key)
}

type Chain []KeyStore

func (c Chain) Load(ctx context.Context) ([]byte, error) {
	for _, ks := range c {
		key, err := ks.Load(ctx)
		if errors.Is(err, ErrNoKey) {
			continue
		}
		return key, err
	}
	return nil, ErrNoKey
}

func (c Chain) Create(ctx context.Context) ([]byte, error) {
	if len(c) == 0 {
		return nil, errors.New("nenhum keystore configurado")
	}
	return c[0].Create(ctx)
}
