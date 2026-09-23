package dotenv

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func sameDocument(a, b Document) bool {
	return a.Description == b.Description && slices.Equal(a.Tags, b.Tags) && slices.Equal(a.Vars, b.Vars)
}

func validDocument(doc Document) error {
	if strings.ContainsAny(doc.Description, "\r\n") {
		return fmt.Errorf("descrição com quebra de linha: %q", doc.Description)
	}
	for _, tag := range doc.Tags {
		if err := ValidateTag(tag); err != nil {
			return err
		}
	}
	seen := map[string]struct{}{}
	for _, v := range doc.Vars {
		if err := ValidateKey(v.Key); err != nil {
			return err
		}
		if _, dup := seen[v.Key]; dup {
			return fmt.Errorf("%w: %q duplicada", ErrInvalidKey, v.Key)
		}
		seen[v.Key] = struct{}{}
	}
	return nil
}

func TestValidateKey(t *testing.T) {
	valid := []string{"A", "_", "database_url", "DATABASE_URL_2", "export"}
	invalid := []string{"", "1KEY", "A-B", "A B", "Á", "A.B"}
	for _, key := range valid {
		if err := ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) = %v", key, err)
		}
	}
	for _, key := range invalid {
		if err := ValidateKey(key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ValidateKey(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
}

func TestValidateTag(t *testing.T) {
	valid := []string{"db", "local-1", "ção", "a.b"}
	invalid := []string{"", "Local DB", "local db", "DB", "a,b", "a\tb"}
	for _, tag := range valid {
		if err := ValidateTag(tag); err != nil {
			t.Errorf("ValidateTag(%q) = %v", tag, err)
		}
	}
	for _, tag := range invalid {
		err := ValidateTag(tag)
		if !errors.Is(err, ErrInvalidTag) {
			t.Errorf("ValidateTag(%q) = %v, want ErrInvalidTag", tag, err)
			continue
		}
		if tag != "" && !strings.Contains(err.Error(), strconv.Quote(tag)) {
			t.Errorf("ValidateTag(%q) error %q does not cite the tag", tag, err)
		}
	}
}
