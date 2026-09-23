package vault

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"a", "postgres-local", "aws.dev_1", "0", strings.Repeat("a", 64)}
	invalid := []string{"", "Postgres Local", "-a", ".a", "a b", "A", strings.Repeat("a", 65), "a/b"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v", name, err)
		}
	}
	for _, name := range invalid {
		if err := ValidateName(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("ValidateName(%q) = %v, want ErrInvalidName", name, err)
		}
	}
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

func TestEnvHelpers(t *testing.T) {
	env := Env{Name: "a", Tags: []string{"db"}, Vars: []Var{{Key: "A", Value: "1"}}}
	if !env.HasTag("db") || env.HasTag("x") {
		t.Fatal("HasTag mismatch")
	}
	if _, ok := env.Lookup("Z"); ok {
		t.Fatal("Lookup found missing key")
	}
	clone := env.Clone()
	clone.Set("A", "2")
	clone.Tags[0] = "changed"
	if value, _ := env.Lookup("A"); value != "1" || env.Tags[0] != "db" {
		t.Fatal("Clone shares memory")
	}
	if env.SameContent(clone) {
		t.Fatal("SameContent true for different envs")
	}
	removed := clone.Unset("A", "missing")
	if strings.Join(removed, ",") != "A" || len(clone.Vars) != 0 {
		t.Fatalf("Unset removed %v, left %v", removed, clone.Vars)
	}
	if !(Env{}).SameContent(Env{Tags: []string{}, Vars: []Var{}}) {
		t.Fatal("nil and empty slices must be the same content")
	}
	if err := (Env{Name: "a", Tags: []string{"db", "ok"}}).Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
