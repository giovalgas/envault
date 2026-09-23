package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/giovalgas/envault/internal/vault"
)

func FuzzParse(f *testing.F) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "*", "*.env"))
	if err != nil {
		f.Fatalf("Glob: %v", err)
	}
	for _, path := range fixtures {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatalf("ReadFile: %v", err)
		}
		f.Add(data, "Postgres local", "db, local", "DATABASE_URL", "postgres://u:p@h/db", "POOL", "10")
	}
	f.Add([]byte("A=\"x\\ny\"\nB='lit'\n"), "", "", "A", "", "A", "a b")
	f.Add([]byte("export A=1 # c\n"), " desc\r\ncom quebra ", "Tag, OUTRA,  ", "1KEY", "\"#$='\\\n\t\r", "export", "abc#def")
	f.Fuzz(func(t *testing.T, raw []byte, description, tags, keyA, valueA, keyB, valueB string) {
		if parsed, err := Parse(raw); err == nil {
			assertRoundTrip(t, parsed)
		}
		first := normalizeKey(keyA)
		second := normalizeKey(keyB)
		if second == first {
			second += "_B"
		}
		assertRoundTrip(t, vault.Env{
			Description: normalizeDescription(description),
			Tags:        normalizeTags(tags),
			Vars:        []vault.Var{{Key: first, Value: valueA}, {Key: second, Value: valueB}},
		})
	})
}

func assertRoundTrip(t *testing.T, env vault.Env) {
	t.Helper()
	if err := env.ValidateContent(); err != nil {
		t.Fatalf("fuzz produced an invalid env: %v", err)
	}
	for _, text := range [][]byte{Format(env), FormatEditor(env)} {
		got, err := Parse(text)
		if err != nil {
			t.Fatalf("Parse(Format(x)) failed: %v\ntext: %q", err, text)
		}
		if !got.SameContent(env) {
			t.Fatalf("Parse(Format(x)) != x\n got: %#v\nwant: %#v\ntext: %q", got, env, text)
		}
	}
}

func normalizeKey(raw string) string {
	key := strings.Map(func(r rune) rune {
		if r == '_' || (r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r))) {
			return r
		}
		return '_'
	}, raw)
	if key == "" || (key[0] >= '0' && key[0] <= '9') {
		key = "K" + key
	}
	return key
}

func normalizeDescription(raw string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, raw))
}

func normalizeTags(raw string) []string {
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		tag := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, strings.ToLower(part))
		if vault.ValidateTag(tag) == nil {
			tags = append(tags, tag)
		}
	}
	return tags
}
