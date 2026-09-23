package domain

import (
	"slices"
	"strings"
	"testing"
)

func TestEditorDiffByKey(t *testing.T) {
	before := Env{Vars: []Var{{Key: "A", Value: "valor-a"}, {Key: "B", Value: "valor-b-antigo"}}}
	after := Env{Vars: []Var{
		{Key: "A", Value: "valor-a"},
		{Key: "B", Value: "valor-b-novo"},
		{Key: "C", Value: "valor-c"},
	}}
	d := Compare(before, after)
	if !slices.Equal(d.Added, []string{"C"}) || len(d.Removed) != 0 || !slices.Equal(d.Changed, []string{"B"}) {
		t.Fatalf("diff = %+v", d)
	}
	if d.Metadata || d.Reordered || d.Empty() {
		t.Fatalf("diff flags = %+v", d)
	}
	text := d.String()
	for _, v := range append(before.Vars, after.Vars...) {
		if strings.Contains(text, v.Value) {
			t.Fatalf("diff text leaks %q: %q", v.Value, text)
		}
	}
	if !strings.Contains(text, "+ C") || !strings.Contains(text, "~ B") {
		t.Fatalf("diff text = %q", text)
	}
}

func TestEditorDiffRemovedAndMetadata(t *testing.T) {
	before := Env{Description: "antes", Tags: []string{"db"}, Vars: []Var{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}}
	after := Env{Description: "depois", Tags: []string{"db"}, Vars: []Var{{Key: "B", Value: "2"}}}
	d := Compare(before, after)
	if !slices.Equal(d.Removed, []string{"A"}) || len(d.Added) != 0 || len(d.Changed) != 0 || !d.Metadata {
		t.Fatalf("diff = %+v", d)
	}
	if !strings.Contains(d.String(), "- A") || !strings.Contains(d.String(), "descrição ou tags") {
		t.Fatalf("diff text = %q", d.String())
	}
}

func TestEditorDiffTagsAndOrder(t *testing.T) {
	before := Env{Tags: []string{"a"}, Vars: []Var{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}}
	after := Env{Tags: []string{"a", "b"}, Vars: []Var{{Key: "B", Value: "2"}, {Key: "A", Value: "1"}}}
	d := Compare(before, after)
	if !d.Metadata || !d.Reordered || len(d.Added)+len(d.Removed)+len(d.Changed) != 0 {
		t.Fatalf("diff = %+v", d)
	}
}

func TestEditorDiffEmpty(t *testing.T) {
	env := Env{Vars: []Var{{Key: "A", Value: "1"}}}
	d := Compare(env, env)
	if !d.Empty() || d.String() != "nenhuma mudança" {
		t.Fatalf("diff = %+v %q", d, d.String())
	}
}
