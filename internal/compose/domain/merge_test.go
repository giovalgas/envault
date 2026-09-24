package domain

import (
	"reflect"
	"slices"
	"testing"
)

func env(name string, pairs ...string) Env {
	e := Env{Name: name}
	for i := 0; i+1 < len(pairs); i += 2 {
		e.Vars = append(e.Vars, Var{Key: pairs[i], Value: pairs[i+1]})
	}
	return e
}

func cloneEnv(e Env) Env {
	return Env{Name: e.Name, Vars: slices.Clone(e.Vars)}
}

func vars(pairs ...string) []Var {
	return env("", pairs...).Vars
}

func tmpl(entries ...TemplateEntry) *Template {
	return &Template{Entries: entries}
}

func withDefault(key, value string) TemplateEntry {
	return TemplateEntry{Key: key, Default: value, HasDefault: true}
}

func bare(key string) TemplateEntry {
	return TemplateEntry{Key: key}
}

func find(t *testing.T, plan Plan, key string) Resolved {
	t.Helper()
	for _, r := range plan.Vars {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("key %s not resolved in %+v", key, plan.Vars)
	return Resolved{}
}

func TestMergeResolve(t *testing.T) {
	cases := []struct {
		name      string
		envs      []Env
		tmpl      *Template
		opts      Options
		pairs     []Var
		conflicts []string
		missing   []string
		extra     []string
	}{
		{
			name:      "last wins",
			envs:      []Env{env("a", "X", "1"), env("b", "X", "2")},
			pairs:     vars("X", "2"),
			conflicts: []string{"X"},
		},
		{
			name:  "no conflict keeps first appearance order",
			envs:  []Env{env("a", "X", "1", "Z", "3"), env("b", "Y", "2")},
			pairs: vars("X", "1", "Z", "3", "Y", "2"),
		},
		{
			name:      "three envs conflict order follows first appearance",
			envs:      []Env{env("a", "B", "1", "A", "1"), env("b", "A", "2"), env("c", "B", "3")},
			pairs:     vars("B", "3", "A", "2"),
			conflicts: []string{"B", "A"},
		},
		{
			name:  "template order",
			envs:  []Env{env("a", "A", "1", "B", "2")},
			tmpl:  tmpl(bare("B"), bare("A")),
			pairs: vars("B", "2", "A", "1"),
		},
		{
			name:  "template default",
			envs:  []Env{env("a", "A", "1")},
			tmpl:  tmpl(withDefault("PORT", "3000"), bare("A")),
			pairs: vars("PORT", "3000", "A", "1"),
		},
		{
			name:  "env overrides template default",
			envs:  []Env{env("a", "PORT", "8080")},
			tmpl:  tmpl(withDefault("PORT", "3000")),
			pairs: vars("PORT", "8080"),
		},
		{
			name:    "missing",
			envs:    []Env{env("a", "A", "1")},
			tmpl:    tmpl(bare("A"), bare("SENTRY_DSN")),
			pairs:   vars("A", "1"),
			missing: []string{"SENTRY_DSN"},
		},
		{
			name:  "empty value in env is not missing",
			envs:  []Env{env("a", "SENTRY_DSN", "")},
			tmpl:  tmpl(bare("SENTRY_DSN")),
			pairs: vars("SENTRY_DSN", ""),
		},
		{
			name:  "extra goes to the end",
			envs:  []Env{env("a", "DEBUG", "1", "A", "1")},
			tmpl:  tmpl(bare("A")),
			pairs: vars("A", "1", "DEBUG", "1"),
			extra: []string{"DEBUG"},
		},
		{
			name:  "only template drops extra",
			envs:  []Env{env("a", "DEBUG", "1", "A", "1")},
			tmpl:  tmpl(bare("A")),
			opts:  Options{OnlyTemplate: true},
			pairs: vars("A", "1"),
			extra: []string{"DEBUG"},
		},
		{
			name:  "only template without template is ignored",
			envs:  []Env{env("a", "DEBUG", "1")},
			opts:  Options{OnlyTemplate: true},
			pairs: vars("DEBUG", "1"),
		},
		{
			name:    "empty template",
			envs:    []Env{env("a", "A", "1")},
			tmpl:    tmpl(),
			pairs:   vars("A", "1"),
			extra:   []string{"A"},
			missing: nil,
		},
		{
			name:  "no envs",
			pairs: vars(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := Resolve(tc.envs, tc.tmpl, tc.opts)
			if got := plan.Pairs(); !slices.Equal(got, tc.pairs) {
				t.Errorf("Pairs = %+v, want %+v", got, tc.pairs)
			}
			check := func(field string, got, want []string) {
				if want == nil {
					want = []string{}
				}
				if got == nil || !slices.Equal(got, want) {
					t.Errorf("%s = %#v, want %#v", field, got, want)
				}
			}
			check("Conflicts", plan.Conflicts, tc.conflicts)
			check("Missing", plan.Missing, tc.missing)
			check("Extra", plan.Extra, tc.extra)
		})
	}
}

func TestMergeResolveOrigins(t *testing.T) {
	plan := Resolve([]Env{env("a", "X", "1", "Y", "2"), env("b", "X", "2")}, nil, Options{})
	if !slices.Equal(plan.Envs, []string{"a", "b"}) {
		t.Fatalf("Envs = %v", plan.Envs)
	}
	x := find(t, plan, "X")
	if x.From != "b" || x.Default || !slices.Equal(x.ShadowNames(), []string{"a"}) {
		t.Fatalf("X = %+v", x)
	}
	if !slices.Equal(x.Shadows, []Source{{Env: "a", Value: "1"}}) {
		t.Fatalf("X.Shadows = %+v", x.Shadows)
	}
	y := find(t, plan, "Y")
	if y.From != "a" || y.Shadows == nil || len(y.Shadows) != 0 {
		t.Fatalf("Y = %+v", y)
	}
}

func TestMergeResolveDefaultHasNoOrigin(t *testing.T) {
	plan := Resolve(nil, tmpl(withDefault("PORT", "3000")), Options{})
	port := find(t, plan, "PORT")
	if !port.Default || port.From != "" || port.Value != "3000" || port.Shadows == nil || len(port.Shadows) != 0 {
		t.Fatalf("PORT = %+v", port)
	}
	if !slices.Equal(plan.Keys(), []string{"PORT"}) {
		t.Fatalf("Keys = %v", plan.Keys())
	}
}

func TestMergeResolveMissingIsNotResolved(t *testing.T) {
	plan := Resolve([]Env{env("a", "A", "1")}, tmpl(bare("SENTRY_DSN")), Options{})
	if slices.Contains(plan.Keys(), "SENTRY_DSN") {
		t.Fatalf("SENTRY_DSN resolved: %+v", plan.Vars)
	}
}

func TestMergeResolveIsPure(t *testing.T) {
	envs := []Env{env("a", "X", "1", "DEBUG", "1"), env("b", "X", "2")}
	template := tmpl(withDefault("PORT", "3000"), bare("X"), bare("MISSING"))
	before := []Env{cloneEnv(envs[0]), cloneEnv(envs[1])}
	first := Resolve(envs, template, Options{})
	second := Resolve(envs, template, Options{})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("plans differ:\n%+v\n%+v", first, second)
	}
	if !reflect.DeepEqual(envs, before) {
		t.Fatalf("envs mutated: %+v", envs)
	}
	first.Vars[1].Shadows[0].Value = "mutated"
	if second.Vars[1].Shadows[0].Value != "1" {
		t.Fatal("plans share shadow storage")
	}
}

func TestMergeTemplateKeys(t *testing.T) {
	if got := tmpl(bare("B"), withDefault("A", "1")).Keys(); !slices.Equal(got, []string{"B", "A"}) {
		t.Fatalf("Keys = %v", got)
	}
}

func TestMergeVars(t *testing.T) {
	cases := []struct {
		name     string
		existing []Var
		incoming []Var
		want     []Var
	}{
		{
			name:     "preserves file order",
			existing: vars("A", "old", "LOCAL", "1"),
			incoming: vars("A", "new", "B", "2"),
			want:     vars("A", "new", "LOCAL", "1", "B", "2"),
		},
		{
			name:     "empty file",
			incoming: vars("A", "1"),
			want:     vars("A", "1"),
		},
		{
			name:     "nothing incoming keeps file",
			existing: vars("LOCAL", "1"),
			want:     vars("LOCAL", "1"),
		},
		{
			name:     "update in the middle",
			existing: vars("A", "1", "B", "2", "C", "3"),
			incoming: vars("C", "30", "B", "20", "D", "4"),
			want:     vars("A", "1", "B", "20", "C", "30", "D", "4"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			existing := slices.Clone(tc.existing)
			got := MergeVars(tc.existing, tc.incoming)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("MergeVars = %+v, want %+v", got, tc.want)
			}
			if !slices.Equal(tc.existing, existing) {
				t.Fatalf("existing mutated: %+v", tc.existing)
			}
		})
	}
}
