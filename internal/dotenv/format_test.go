package dotenv

import (
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

func TestFormatValue(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"abc", "abc"},
		{"a b", `"a b"`},
		{"", `""`},
		{"a#b", `"a#b"`},
		{`a"b`, `"a\"b"`},
		{"a'b", `"a'b"`},
		{"$HOME", `"$HOME"`},
		{"a=b", `"a=b"`},
		{"x\ny", `"x\ny"`},
		{"x\r\ty", `"x\r\ty"`},
		{`C:\dir`, `"C:\\dir"`},
		{"`cmd`", "\"`cmd`\""},
		{"postgres://u:p@h:5432/db", "postgres://u:p@h:5432/db"},
	}
	for _, tc := range cases {
		if got := FormatValue(tc.value); got != tc.want {
			t.Errorf("FormatValue(%q) = %s, want %s", tc.value, got, tc.want)
		}
	}
}

func TestFormatLines(t *testing.T) {
	cases := []struct {
		name string
		env  vault.Env
		want string
	}{
		{"simples", vault.Env{Vars: vars("A", "abc")}, "A=abc\n"},
		{"com espaço", vault.Env{Vars: vars("A", "a b")}, "A=\"a b\"\n"},
		{"vazio", vault.Env{Vars: vars("A", "")}, "A=\"\"\n"},
		{"ordem", vault.Env{Vars: vars("B", "2", "A", "1")}, "B=2\nA=1\n"},
		{
			"metadados",
			vault.Env{Description: "Postgres local", Tags: []string{"db", "local"}, Vars: vars("A", "1")},
			"# @description: Postgres local\n# @tags: db, local\n\nA=1\n",
		},
		{"só tags", vault.Env{Tags: []string{"db"}}, "# @tags: db\n"},
		{"nada", vault.Env{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(Format(tc.env)); got != tc.want {
				t.Fatalf("Format = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatEditorMatchesFixture(t *testing.T) {
	env := vault.Env{
		Description: "Postgres local via docker-compose, porta 5432",
		Tags:        []string{"db", "local"},
		Vars: vars(
			"DATABASE_URL", "postgres://user:pass@localhost:5432/app",
			"DATABASE_POOL", "10",
		),
	}
	if got, want := string(FormatEditor(env)), string(readFixture(t, "valid/editor.env")); got != want {
		t.Fatalf("FormatEditor =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatEditorEmptyEnv(t *testing.T) {
	got := string(FormatEditor(vault.Env{}))
	if !strings.HasPrefix(got, "# @description:\n# @tags:\n#\n") {
		t.Fatalf("FormatEditor = %q", got)
	}
	if !strings.Contains(got, EditorInstructions) {
		t.Fatal("instructions missing")
	}
	parsed, err := Parse([]byte(got))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !parsed.SameContent(vault.Env{}) {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestRoundTripExamples(t *testing.T) {
	envs := []vault.Env{
		{Description: "desc com # e \"aspas\"", Tags: []string{"a", "b-c"}, Vars: vars(
			"A", "", "B", " espaço ", "C", "x\ny\r\n", "D", `\n literal`, "E", "'simples'", "F", "#", "G", "$X=${Y}",
		)},
		{Vars: vars("export", "1", "K", "\x00\x01\x7f", "L", "ção ☃")},
	}
	for _, env := range envs {
		for _, text := range [][]byte{Format(env), FormatEditor(env)} {
			got, err := Parse(text)
			if err != nil {
				t.Fatalf("Parse(%q): %v", text, err)
			}
			if !got.SameContent(env) {
				t.Fatalf("round trip mismatch:\n got %+v\nwant %+v\ntext %q", got, env, text)
			}
		}
	}
}
