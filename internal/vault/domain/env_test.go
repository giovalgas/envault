package domain

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

func TestDocumentRoundTrip(t *testing.T) {
	env := Env{Name: "a", Description: "d", Tags: []string{"db"}, Vars: []Var{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}}
	doc := env.Document()
	if doc.Description != "d" || len(doc.Tags) != 1 || len(doc.Vars) != 2 || doc.Vars[1].Key != "B" {
		t.Fatalf("Document = %+v", doc)
	}
	doc.Tags[0] = "changed"
	if env.Tags[0] != "db" {
		t.Fatal("Document shares tags with env")
	}
	back := FromDocument(env.Document())
	if back.Name != "" || !back.SameContent(env) {
		t.Fatalf("FromDocument = %+v", back)
	}
	if empty := FromDocument(Env{}.Document()); empty.Vars != nil || empty.Tags != nil {
		t.Fatalf("empty conversion = %+v", empty)
	}
}

func TestValidateContentRejects(t *testing.T) {
	cases := map[string]Env{
		"descrição":      {Description: "a\nb"},
		"tag":            {Tags: []string{"Bad Tag"}},
		"chave":          {Vars: []Var{{Key: "1A", Value: "x"}}},
		"chave repetida": {Vars: []Var{{Key: "A", Value: "1"}, {Key: "A", Value: "2"}}},
	}
	for name, env := range cases {
		if err := env.ValidateContent(); err == nil {
			t.Errorf("%s: ValidateContent aceitou %+v", name, env)
		}
	}
	if err := (Env{Name: "Bad"}).Validate(); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("Validate err = %v", err)
	}
}
