package dotenv

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	return data
}

func vars(pairs ...string) []vault.Var {
	out := make([]vault.Var, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, vault.Var{Key: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func TestParseValidFixtures(t *testing.T) {
	cases := []struct {
		file string
		want vault.Env
	}{
		{
			file: "valid/editor.env",
			want: vault.Env{
				Description: "Postgres local via docker-compose, porta 5432",
				Tags:        []string{"db", "local"},
				Vars: vars(
					"DATABASE_URL", "postgres://user:pass@localhost:5432/app",
					"DATABASE_POOL", "10",
				),
			},
		},
		{
			file: "valid/syntax.env",
			want: vault.Env{Vars: vars(
				"EXPORTED", "1",
				"SINGLE", `x\ny`,
				"DOUBLE", "x\ny",
				"ESCAPES", "tab\there \"quoted\" back\\slash",
				"INLINE", "abc",
				"GLUED", "abc#def",
				"EMPTY", "",
				"QUOTED_EMPTY", "",
				"SPACED", "value with spaces",
				"AFTER_QUOTE", "kept",
				"HASH_ONLY", "",
				"INDENTED", "yes",
				"URL", "https://example.com/?a=1&b=2",
			)},
		},
		{
			file: "valid/crlf.env",
			want: vault.Env{Vars: vars("CRLF", "1", "OTHER", "two")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got, err := Parse(readFixture(t, tc.file))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			assertSameEnv(t, got, tc.want)
		})
	}
}

func TestParseInvalidFixtures(t *testing.T) {
	cases := []struct {
		file   string
		line   int
		reason []string
	}{
		{"invalid/invalid_key.env", 4, []string{"chave inválida", `"1KEY"`}},
		{"invalid/unclosed_double.env", 2, []string{"aspas não fechadas", `"B"`}},
		{"invalid/unclosed_single.env", 1, []string{"aspas não fechadas"}},
		{"invalid/trailing_backslash.env", 1, []string{"aspas não fechadas"}},
		{"invalid/duplicate_key.env", 3, []string{"chave duplicada", `"A"`}},
		{"invalid/missing_equals.env", 2, []string{"'='"}},
		{"invalid/bad_tag.env", 1, []string{"tag inválida", "Local DB"}},
		{"invalid/tail_after_quote.env", 1, []string{"depois das aspas"}},
		{"invalid/duplicate_description.env", 2, []string{"@description"}},
		{"invalid/duplicate_tags.env", 2, []string{"@tags"}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			_, err := Parse(readFixture(t, tc.file))
			assertParseError(t, err, tc.line, tc.reason...)
		})
	}
}

func TestParseScenarios(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  vault.Env
	}{
		{"metadados", "# @description: Postgres local\n# @tags: db, local\n", vault.Env{Description: "Postgres local", Tags: []string{"db", "local"}}},
		{"comentário comum", "# qualquer coisa\nA=1\n", vault.Env{Vars: vars("A", "1")}},
		{"export", "export A=1", vault.Env{Vars: vars("A", "1")}},
		{"export com tab", "export\tA=1", vault.Env{Vars: vars("A", "1")}},
		{"chave export", "export=1", vault.Env{Vars: vars("export", "1")}},
		{"aspas simples literais", `A='x\ny'`, vault.Env{Vars: vars("A", `x\ny`)}},
		{"aspas duplas com escape", `A="x\ny"`, vault.Env{Vars: vars("A", "x\ny")}},
		{"escape desconhecido preservado", `A="x\qy"`, vault.Env{Vars: vars("A", `x\qy`)}},
		{"comentário inline", "A=abc # nota", vault.Env{Vars: vars("A", "abc")}},
		{"hash colado", "A=abc#def", vault.Env{Vars: vars("A", "abc#def")}},
		{"hash inicial colado", "A=#abc", vault.Env{Vars: vars("A", "#abc")}},
		{"comentário após aspas simples", "A='x' # c", vault.Env{Vars: vars("A", "x")}},
		{"metadado vazio", "# @description:\n# @tags:\n", vault.Env{}},
		{"tags com vazios", "# @tags: a,, b ,", vault.Env{Tags: []string{"a", "b"}}},
		{"bom", "\ufeffA=1", vault.Env{Vars: vars("A", "1")}},
		{"vazio", "", vault.Env{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.input))
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.input, err)
			}
			assertSameEnv(t, got, tc.want)
		})
	}
}

func TestParseErrorsDoNotLeakValues(t *testing.T) {
	inputs := []string{
		"A=\"supersegredo",
		"A='supersegredo",
		"A=\"x\" supersegredo",
		"supersegredo",
		"A=supersegredo\nA=supersegredo",
	}
	for _, input := range inputs {
		_, err := Parse([]byte(input))
		if err == nil {
			t.Fatalf("Parse(%q) succeeded", input)
		}
		if strings.Contains(err.Error(), "supersegredo") {
			t.Fatalf("error %q leaks the value", err)
		}
	}
}

func TestParseErrorIsSyntax(t *testing.T) {
	_, err := Parse([]byte("1KEY=x"))
	if !errors.Is(err, ErrSyntax) {
		t.Fatalf("err = %v, want ErrSyntax", err)
	}
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Line != 1 {
		t.Fatalf("err = %#v", err)
	}
	if got := err.Error(); got != `linha 1: chave inválida "1KEY"` {
		t.Fatalf("Error() = %q", got)
	}
}

func TestIsBlank(t *testing.T) {
	blank := []string{"", "\n\n", "# a\n  # b\n", string(FormatEditor(vault.Env{Description: "x", Tags: []string{"a"}}))}
	for _, input := range blank {
		if !IsBlank([]byte(input)) {
			t.Errorf("IsBlank(%q) = false", input)
		}
	}
	for _, input := range []string{"A=1", "# c\nlixo"} {
		if IsBlank([]byte(input)) {
			t.Errorf("IsBlank(%q) = true", input)
		}
	}
}

func assertSameEnv(t *testing.T, got, want vault.Env) {
	t.Helper()
	if got.Description != want.Description {
		t.Errorf("Description = %q, want %q", got.Description, want.Description)
	}
	if !slices.Equal(got.Tags, want.Tags) {
		t.Errorf("Tags = %q, want %q", got.Tags, want.Tags)
	}
	if !slices.Equal(got.Vars, want.Vars) {
		t.Errorf("Vars = %q, want %q", got.Vars, want.Vars)
	}
}

func assertParseError(t *testing.T, err error, line int, fragments ...string) {
	t.Helper()
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *ParseError", err)
	}
	if pe.Line != line {
		t.Fatalf("Line = %d, want %d (%v)", pe.Line, line, err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error %q does not contain %q", err, fragment)
		}
	}
}
