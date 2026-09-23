package crypto

import (
	"bytes"
	"errors"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("sem entropia") }

func mustKey(t *testing.T) []byte {
	t.Helper()
	key, err := NewKey()
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	return key
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := mustKey(t)
	for _, plaintext := range [][]byte{nil, []byte(""), []byte("{\"schema_version\":1}"), bytes.Repeat([]byte{0, 1, 2}, 4096)} {
		sealed, err := Seal(key, plaintext)
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		if !bytes.HasPrefix(sealed, Header()) {
			t.Fatalf("sealed data lacks header")
		}
		opened, err := Open(key, sealed)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Fatalf("Open = %q, want %q", opened, plaintext)
		}
	}
}

func TestLayout(t *testing.T) {
	sealed, err := Seal(mustKey(t), []byte("abc"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if string(sealed[:7]) != "ENVAULT" || sealed[7] != 0x01 {
		t.Fatalf("header = %q", sealed[:8])
	}
	if len(sealed) != HeaderSize+NonceSize+3+16 {
		t.Fatalf("len = %d", len(sealed))
	}
}

func TestTamperedHeaderFails(t *testing.T) {
	key := mustKey(t)
	sealed, err := Seal(key, []byte("segredo"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	tamperedVersion := bytes.Clone(sealed)
	tamperedVersion[len(Magic)] = 0x02
	if _, err := Open(key, tamperedVersion); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("version tamper err = %v, want ErrDecrypt", err)
	}
	tamperedMagic := bytes.Clone(sealed)
	tamperedMagic[0] = 'X'
	if _, err := Open(key, tamperedMagic); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("magic tamper err = %v, want ErrDecrypt", err)
	}
	tamperedBody := bytes.Clone(sealed)
	tamperedBody[len(tamperedBody)-1] ^= 0xff
	if _, err := Open(key, tamperedBody); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("body tamper err = %v, want ErrDecrypt", err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	sealed, err := Seal(mustKey(t), []byte("segredo"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := Open(mustKey(t), sealed); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestShortDataFails(t *testing.T) {
	key := mustKey(t)
	for _, data := range [][]byte{nil, []byte("ENVAULT"), append(Header(), make([]byte, NonceSize)...)} {
		if _, err := Open(key, data); !errors.Is(err, ErrDecrypt) {
			t.Fatalf("Open(%d bytes) err = %v, want ErrDecrypt", len(data), err)
		}
	}
}

func TestNonceNeverRepeats(t *testing.T) {
	key := mustKey(t)
	first, err := Seal(key, []byte("mesmo"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	second, err := Seal(key, []byte("mesmo"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	nonceOf := func(b []byte) []byte { return b[HeaderSize : HeaderSize+NonceSize] }
	if bytes.Equal(nonceOf(first), nonceOf(second)) {
		t.Fatal("nonce repeated")
	}
}

func TestKeySizeValidated(t *testing.T) {
	if _, err := Seal(make([]byte, 16), nil); !errors.Is(err, ErrKeySize) {
		t.Fatalf("Seal err = %v, want ErrKeySize", err)
	}
	if _, err := Open(make([]byte, 31), make([]byte, 64)); !errors.Is(err, ErrKeySize) {
		t.Fatalf("Open err = %v, want ErrKeySize", err)
	}
}

func TestNewKey(t *testing.T) {
	a, b := mustKey(t), mustKey(t)
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
	if _, err := seal(failingReader{}, make([]byte, KeySize), nil); err == nil {
		t.Fatal("seal succeeded with failing reader")
	}
}
