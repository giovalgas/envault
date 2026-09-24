package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var baseTime = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func sampleEnv(name string) Env {
	return Env{
		Name:        name,
		Description: "Postgres local",
		Tags:        []string{"db", "local"},
		Vars: []Var{
			{Key: "B", Value: "2"},
			{Key: "A", Value: "1"},
			{Key: "C", Value: "3"},
		},
	}
}

func mustCreate(t *testing.T, s *Snapshot, env Env, now time.Time) Env {
	t.Helper()
	created, err := s.Create(env, now)
	if err != nil {
		t.Fatalf("Create(%s): %v", env.Name, err)
	}
	return created
}

func TestSnapshotCreateGetPreservesOrder(t *testing.T) {
	s := NewSnapshot(baseTime)
	created := mustCreate(t, s, sampleEnv("a"), baseTime)
	if !created.CreatedAt.Equal(baseTime) || !created.UpdatedAt.Equal(baseTime) {
		t.Fatalf("timestamps = %v, %v", created.CreatedAt, created.UpdatedAt)
	}
	env, err := s.Get("a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := strings.Join(env.Keys(), ","); got != "B,A,C" {
		t.Fatalf("keys = %s, want B,A,C", got)
	}
	if env.Name != "a" || !env.SameContent(sampleEnv("a")) {
		t.Fatalf("env = %+v", env)
	}
	if !s.Has("a") || s.Has("b") {
		t.Fatal("Has mismatch")
	}
}

func TestSnapshotCreateValidation(t *testing.T) {
	cases := []struct {
		name string
		env  Env
		want error
		cite string
	}{
		{"nome inválido", Env{Name: "Postgres Local"}, ErrInvalidName, "Postgres Local"},
		{"chave duplicada", Env{Name: "a", Vars: []Var{{Key: "A", Value: "1"}, {Key: "A", Value: "2"}}}, ErrInvalidKey, "A"},
		{"chave inválida", Env{Name: "a", Vars: []Var{{Key: "1KEY", Value: "x"}}}, ErrInvalidKey, "1KEY"},
		{"tag com espaço", Env{Name: "a", Tags: []string{"Local DB"}}, ErrInvalidTag, "Local DB"},
		{"descrição multilinha", Env{Name: "a", Description: "a\nb"}, ErrInvalidDescription, "quebra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSnapshot(baseTime)
			_, err := s.Create(tc.env, baseTime)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.cite) {
				t.Fatalf("err %q does not cite %q", err, tc.cite)
			}
			if len(s.Envs) != 0 {
				t.Fatal("invalid create changed the snapshot")
			}
		})
	}
}

func TestSnapshotErrorsDoNotLeakValues(t *testing.T) {
	_, err := NewSnapshot(baseTime).Create(Env{Name: "a", Vars: []Var{{Key: "A", Value: "supersegredo"}, {Key: "A", Value: "supersegredo"}}}, baseTime)
	if err == nil || strings.Contains(err.Error(), "supersegredo") {
		t.Fatalf("err = %v", err)
	}
}

func TestSnapshotCreateDuplicate(t *testing.T) {
	s := NewSnapshot(baseTime)
	mustCreate(t, s, sampleEnv("a"), baseTime)
	if _, err := s.Create(sampleEnv("a"), baseTime); !errors.Is(err, ErrEnvExists) {
		t.Fatalf("err = %v, want ErrEnvExists", err)
	}
}

func TestSnapshotGetMissing(t *testing.T) {
	_, err := NewSnapshot(baseTime).Get("nao-existe")
	if !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("err = %v, want ErrEnvNotFound", err)
	}
	if !strings.Contains(err.Error(), "nao-existe") {
		t.Fatalf("err %q does not cite name", err)
	}
}

func TestSnapshotPut(t *testing.T) {
	s := NewSnapshot(baseTime)
	put, err := s.Put(Env{Name: "b", Vars: []Var{{Key: "Y", Value: "1"}}}, baseTime)
	if err != nil {
		t.Fatalf("Put new: %v", err)
	}
	later := baseTime.Add(time.Hour)
	again, err := s.Put(Env{Name: "b", Vars: []Var{{Key: "Z", Value: "2"}}}, later)
	if err != nil {
		t.Fatalf("Put existing: %v", err)
	}
	if !again.CreatedAt.Equal(put.CreatedAt) || !again.UpdatedAt.Equal(later) || strings.Join(again.Keys(), ",") != "Z" {
		t.Fatalf("Put existing = %+v", again)
	}
	if _, err := s.Put(Env{Name: "Bad"}, later); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("Put invalid err = %v", err)
	}
}

func TestSnapshotModify(t *testing.T) {
	s := NewSnapshot(baseTime)
	mustCreate(t, s, sampleEnv("a"), baseTime)
	later := baseTime.Add(time.Minute)
	env, err := s.Modify("a", later, func(e *Env) error {
		e.Set("A", "10")
		e.Set("D", "4")
		e.Unset("B")
		return nil
	})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if got := strings.Join(env.Keys(), ","); got != "A,C,D" {
		t.Fatalf("keys = %s", got)
	}
	if value, _ := env.Lookup("A"); value != "10" {
		t.Fatalf("A = %q", value)
	}
	if !env.UpdatedAt.Equal(later) || !env.CreatedAt.Equal(baseTime) {
		t.Fatal("Modify timestamps wrong")
	}
	if _, err := s.Modify("x", later, func(*Env) error { return nil }); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	boom := errors.New("boom")
	if _, err := s.Modify("a", later, func(*Env) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("change err = %v", err)
	}
	if _, err := s.Modify("a", later, func(e *Env) error { e.Set("1BAD", "x"); return nil }); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key err = %v", err)
	}
	if stored, _ := s.Get("a"); strings.Join(stored.Keys(), ",") != "A,C,D" {
		t.Fatalf("failed Modify changed the snapshot: %v", stored.Keys())
	}
}

func TestSnapshotRename(t *testing.T) {
	s := NewSnapshot(baseTime)
	mustCreate(t, s, sampleEnv("a"), baseTime)
	mustCreate(t, s, sampleEnv("b"), baseTime)
	later := baseTime.Add(time.Hour)
	if _, err := s.Rename("a", "b", later); !errors.Is(err, ErrEnvExists) {
		t.Fatalf("occupied err = %v, want ErrEnvExists", err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := s.Get(name); err != nil {
			t.Fatalf("env %s lost after failed rename: %v", name, err)
		}
	}
	renamed, err := s.Rename("a", "c", later)
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Name != "c" || !renamed.UpdatedAt.Equal(later) {
		t.Fatalf("renamed = %+v", renamed)
	}
	if _, err := s.Get("a"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("old name still exists: %v", err)
	}
	same, err := s.Rename("c", "c", later)
	if err != nil || same.Name != "c" {
		t.Fatalf("same-name rename = %+v, %v", same, err)
	}
	if _, err := s.Rename("nope", "d", later); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := s.Rename("c", "Bad Name", later); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid err = %v", err)
	}
}

func TestSnapshotCopy(t *testing.T) {
	s := NewSnapshot(baseTime)
	original := mustCreate(t, s, sampleEnv("a"), baseTime)
	later := baseTime.Add(time.Hour)
	copied, err := s.Copy("a", "c", later)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if !copied.SameContent(original) || copied.Name != "c" {
		t.Fatal("copy content differs")
	}
	if !copied.CreatedAt.After(original.CreatedAt) {
		t.Fatal("copy did not get a new created_at")
	}
	if _, err := s.Copy("a", "c", later); !errors.Is(err, ErrEnvExists) {
		t.Fatalf("dst exists err = %v", err)
	}
	if _, err := s.Copy("x", "y", later); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("missing src err = %v", err)
	}
	if _, err := s.Copy("a", "Y Y", later); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid dst err = %v", err)
	}
	copied.Vars[0].Value = "mutated"
	stored, err := s.Get("a")
	if err != nil || stored.Vars[0].Value == "mutated" {
		t.Fatal("copy shares memory with source")
	}
}

func TestSnapshotDelete(t *testing.T) {
	s := NewSnapshot(baseTime)
	mustCreate(t, s, sampleEnv("a"), baseTime)
	if err := s.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("a"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("Get after delete err = %v", err)
	}
	if err := s.Delete("a"); !errors.Is(err, ErrEnvNotFound) {
		t.Fatalf("Delete missing err = %v", err)
	}
}

func TestSnapshotSortedAndValidate(t *testing.T) {
	s := NewSnapshot(baseTime)
	for _, name := range []string{"stripe-test", "aws-dev", "postgres-local"} {
		mustCreate(t, s, sampleEnv(name), baseTime)
	}
	var names []string
	for _, env := range s.Sorted() {
		names = append(names, env.Name)
	}
	if got := strings.Join(names, ","); got != "aws-dev,postgres-local,stripe-test" {
		t.Fatalf("names = %s", got)
	}
	s.Envs["ok"] = Env{Name: "ignored"}
	if err := s.Validate(); err != nil || s.Envs["ok"].Name != "ok" {
		t.Fatalf("Validate = %v, %+v", err, s.Envs["ok"])
	}
	s.Envs["Bad Name"] = Env{}
	if err := s.Validate(); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("Validate err = %v", err)
	}
}
