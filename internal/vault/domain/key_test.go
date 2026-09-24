package domain

import (
	"bytes"
	"errors"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("sem entropia") }

func TestNewKey(t *testing.T) {
	a, err := NewKey()
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	b, err := NewKey()
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	if len(a) != KeySize {
		t.Fatalf("len = %d", len(a))
	}
	if bytes.Equal(a, b) {
		t.Fatal("two keys are equal")
	}
}

func TestRandomnessFailure(t *testing.T) {
	if _, err := newKey(failingReader{}); err == nil {
		t.Fatal("newKey succeeded with failing reader")
	}
}
