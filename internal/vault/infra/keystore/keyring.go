package keystore

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/giovalgas/envault/internal/vault/domain"
)

const KeyringService = "envault"

type Keyring struct {
	service string
	account string
}

func NewKeyring(account string) *Keyring {
	return &Keyring{service: KeyringService, account: account}
}

func (k *Keyring) Service() string {
	return k.service
}

func (k *Keyring) Account() string {
	return k.account
}

func (k *Keyring) Load(ctx context.Context) ([]byte, error) {
	encoded, err := k.get(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrKeyringUnavailable) {
			return nil, fmt.Errorf("%w: %w", domain.ErrNoKey, err)
		}
		return nil, err
	}
	key, err := DecodeKey(encoded)
	if err != nil {
		return nil, fmt.Errorf("keychain %s/%s: %w", k.service, k.account, err)
	}
	return key, nil
}

func (k *Keyring) Create(ctx context.Context) ([]byte, error) {
	key, err := domain.NewKey()
	if err != nil {
		return nil, err
	}
	if err := k.Save(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (k *Keyring) Save(ctx context.Context, key []byte) error {
	if len(key) != domain.KeySize {
		return fmt.Errorf("%w: esperado %d bytes, recebido %d", domain.ErrMalformedKey, domain.KeySize, len(key))
	}
	existing, err := k.get(ctx)
	switch {
	case err == nil:
		current, decodeErr := DecodeKey(existing)
		if decodeErr == nil && bytes.Equal(current, key) {
			return nil
		}
		return fmt.Errorf("%w: keychain %s/%s já guarda outra chave", domain.ErrKeyExists, k.service, k.account)
	case !errors.Is(err, domain.ErrNoKey):
		return err
	}
	if err := keyring.Set(k.service, k.account, EncodeKey(key)); err != nil {
		return k.unavailable(err)
	}
	return nil
}

func (k *Keyring) Remove(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := keyring.Delete(k.service, k.account)
	if errors.Is(err, keyring.ErrNotFound) {
		return domain.ErrNoKey
	}
	if err != nil {
		return k.unavailable(err)
	}
	return nil
}

func (k *Keyring) get(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	encoded, err := keyring.Get(k.service, k.account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", domain.ErrNoKey
	}
	if err != nil {
		return "", k.unavailable(err)
	}
	return encoded, nil
}

func (k *Keyring) unavailable(err error) error {
	return fmt.Errorf("%w (serviço %s, conta %s): %w", domain.ErrKeyringUnavailable, k.service, k.account, err)
}
