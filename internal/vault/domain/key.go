package domain

import (
	"crypto/rand"
	"fmt"
	"io"
)

const KeySize = 32

func NewKey() ([]byte, error) {
	return newKey(rand.Reader)
}

func newKey(random io.Reader) ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(random, key); err != nil {
		return nil, fmt.Errorf("gerar chave: %w", err)
	}
	return key, nil
}
