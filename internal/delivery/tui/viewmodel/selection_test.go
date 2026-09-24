package viewmodel

import (
	"slices"
	"testing"

	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

func TestSelectionToggleKeepsMarkingOrder(t *testing.T) {
	s := NewSelection(nil).Toggle("stripe-test").Toggle("postgres-local").Toggle("redis")
	if !slices.Equal(s.Names(), []string{"stripe-test", "postgres-local", "redis"}) {
		t.Fatalf("seleção = %v", s.Names())
	}
	s = s.Toggle("postgres-local")
	if !slices.Equal(s.Names(), []string{"stripe-test", "redis"}) {
		t.Fatalf("desmarcar = %v", s.Names())
	}
	if s.Position("redis") != 2 || s.Position("postgres-local") != 0 || !s.Contains("stripe-test") {
		t.Fatalf("posições erradas em %v", s.Names())
	}
}

func TestSelectionIsImmutable(t *testing.T) {
	names := []string{"a", "b"}
	s := NewSelection(names)
	names[0] = "mudou"
	s.Names()[1] = "mudou"
	_ = s.Toggle("c")
	_ = s.Rename("a", "z")
	_, _ = s.Move(0, 1)
	if !slices.Equal(s.Names(), []string{"a", "b"}) {
		t.Fatalf("seleção alterada por fora: %v", s.Names())
	}
}

func TestSelectionKeepRenameMove(t *testing.T) {
	s := NewSelection([]string{"redis", "sumiu", "postgres-local"})
	s = s.Keep(func(name string) bool { return name != "sumiu" })
	if !slices.Equal(s.Names(), []string{"redis", "postgres-local"}) {
		t.Fatalf("keep = %v", s.Names())
	}
	s = s.Rename("postgres-local", "pg").Rename("inexistente", "x")
	if !slices.Equal(s.Names(), []string{"redis", "pg"}) {
		t.Fatalf("rename = %v", s.Names())
	}
	moved, ok := s.Move(0, 1)
	if !ok || !slices.Equal(moved.Names(), []string{"pg", "redis"}) {
		t.Fatalf("move = %v %v", moved.Names(), ok)
	}
	for _, tt := range []struct{ index, delta int }{{0, -1}, {1, 1}, {5, -1}, {-1, 1}} {
		if _, ok := s.Move(tt.index, tt.delta); ok {
			t.Errorf("Move(%d, %d) deveria ser recusado", tt.index, tt.delta)
		}
	}
}

func TestSelectedEnvsLineAndLabels(t *testing.T) {
	if text, empty := NewSelection(nil).SelectedEnvsLine(); !empty || text != "SELECTED_ENVS=" {
		t.Fatalf("linha vazia = %q %v", text, empty)
	}
	if text, empty := NewSelection([]string{"stripe-test"}).SelectedEnvsLine(); empty || text != "SELECTED_ENVS=stripe-test" {
		t.Fatalf("linha com uma env = %q", text)
	}
	s := NewSelection([]string{"stripe-test", "postgres-local"})
	if text, empty := s.SelectedEnvsLine(); empty || text != "SELECTED_ENVS=stripe-test,postgres-local" {
		t.Fatalf("linha com várias envs = %q", text)
	}
	if label, marked := s.MarkLabel("postgres-local"); !marked || label != "[2]" {
		t.Fatalf("label marcada = %q %v", label, marked)
	}
	if label, marked := s.MarkLabel("redis"); marked || label != MarkOff {
		t.Fatalf("label desmarcada = %q %v", label, marked)
	}
	many := NewSelection([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"})
	if label, _ := many.MarkLabel("a"); label != "[1] " {
		t.Fatalf("label com dez marcadas = %q", label)
	}
	if label, _ := many.MarkLabel("z"); label != MarkOff+" " {
		t.Fatalf("label desmarcada com dez = %q", label)
	}
}

func TestSelectionPickFollowsOrder(t *testing.T) {
	envs := []vaultusecase.EnvView{{Name: "a", Tags: []string{"t"}}, {Name: "b"}, {Name: "c"}}
	picked := NewSelection([]string{"c", "sumiu", "a"}).Pick(envs)
	if len(picked) != 2 || picked[0].Name != "c" || picked[1].Name != "a" {
		t.Fatalf("pick = %+v", picked)
	}
	picked[1].Tags[0] = "mudou"
	if envs[0].Tags[0] != "t" {
		t.Fatal("pick deveria clonar as envs")
	}
}
