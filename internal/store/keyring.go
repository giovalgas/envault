package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/giovalgas/envault/internal/crypto"
)

const KeyringService = "envault"

var ErrKeyringUnavailable = errors.New("keychain do sistema indisponível")

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
		if errors.Is(err, ErrKeyringUnavailable) {
			return nil, fmt.Errorf("%w: %w", ErrNoKey, err)
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
	key, err := crypto.NewKey()
	if err != nil {
		return nil, err
	}
	if err := k.Save(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (k *Keyring) Save(ctx context.Context, key []byte) error {
	if len(key) != crypto.KeySize {
		return fmt.Errorf("%w: esperado %d bytes, recebido %d", ErrMalformedKey, crypto.KeySize, len(key))
	}
	existing, err := k.get(ctx)
	switch {
	case err == nil:
		current, decodeErr := DecodeKey(existing)
		if decodeErr == nil && bytes.Equal(current, key) {
			return nil
		}
		return fmt.Errorf("%w: keychain %s/%s já guarda outra chave", ErrKeyExists, k.service, k.account)
	case !errors.Is(err, ErrNoKey):
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
		return ErrNoKey
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
		return "", ErrNoKey
	}
	if err != nil {
		return "", k.unavailable(err)
	}
	return encoded, nil
}

func (k *Keyring) unavailable(err error) error {
	return fmt.Errorf("%w (serviço %s, conta %s): %w", ErrKeyringUnavailable, k.service, k.account, err)
}
