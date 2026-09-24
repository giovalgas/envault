package dotenv

import (
	"slices"
	"strings"
	"testing"
)

func TestParseTemplateFixture(t *testing.T) {
	tmpl, err := ParseTemplate(readFixture(t, "template.env.example"))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	want := []TemplateEntry{
		{Key: "PORT", Default: "3000", HasDefault: true},
		{Key: "SENTRY_DSN"},
		{Key: "APP_URL", Default: "http://localhost:3000", HasDefault: true},
		{Key: "EMPTY_QUOTED"},
	}
	if !slices.Equal(tmpl.Entries, want) {
		t.Fatalf("Entries = %+v, want %+v", tmpl.Entries, want)
	}
	if got := strings.Join(tmpl.Keys(), ","); got != "PORT,SENTRY_DSN,APP_URL,EMPTY_QUOTED" {
		t.Fatalf("Keys = %s", got)
	}
	port, ok := tmpl.Lookup("PORT")
	if !ok || port.Default != "3000" {
		t.Fatalf("Lookup(PORT) = %+v, %v", port, ok)
	}
	if _, ok := tmpl.Lookup("NOPE"); ok {
		t.Fatal("Lookup found missing key")
	}
}

func TestParseTemplateIgnoresMetadata(t *testing.T) {
	tmpl, err := ParseTemplate([]byte("# @tags: Not Valid\n# @description: x\nA=1\n"))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	if len(tmpl.Entries) != 1 {
		t.Fatalf("Entries = %+v", tmpl.Entries)
	}
}

func TestParseTemplateErrors(t *testing.T) {
	cases := []struct {
		input string
		line  int
		frag  string
	}{
		{"A=1\nA=2\n", 2, "chave duplicada"},
		{"\n\n1X=\n", 3, "chave inválida"},
		{"A=\"x\n", 1, "aspas não fechadas"},
	}
	for _, tc := range cases {
		_, err := ParseTemplate([]byte(tc.input))
		assertParseError(t, err, tc.line, tc.frag)
	}
}
