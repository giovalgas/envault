package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	Magic      = "ENVAULT"
	Version    = byte(0x01)
	KeySize    = 32
	NonceSize  = 12
	HeaderSize = len(Magic) + 1
)

var (
	ErrDecrypt = errors.New("falha ao decifrar o cofre")
	ErrKeySize = errors.New("chave precisa ter 32 bytes")
)

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

func Header() []byte {
	return append([]byte(Magic), Version)
}

func Seal(key, plaintext []byte) ([]byte, error) {
	return seal(rand.Reader, key, plaintext)
}

func seal(random io.Reader, key, plaintext []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	header := Header()
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("gerar nonce: %w", err)
	}
	out := make([]byte, 0, HeaderSize+NonceSize+len(plaintext)+aead.Overhead())
	out = append(out, header...)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plaintext, header), nil
}

func Open(key, data []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(data) < HeaderSize+NonceSize+aead.Overhead() {
		return nil, fmt.Errorf("%w: arquivo curto demais", ErrDecrypt)
	}
	header := data[:HeaderSize]
	if string(header[:len(Magic)]) != Magic {
		return nil, fmt.Errorf("%w: cabeçalho desconhecido", ErrDecrypt)
	}
	nonce := data[HeaderSize : HeaderSize+NonceSize]
	plaintext, err := aead.Open(nil, nonce, data[HeaderSize+NonceSize:], header)
	if err != nil {
		return nil, fmt.Errorf("%w: chave errada ou conteúdo adulterado", ErrDecrypt)
	}
	return plaintext, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: recebido %d", ErrKeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("criar cifra: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("criar GCM: %w", err)
	}
	return aead, nil
}
