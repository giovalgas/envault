package domain

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const sourcePath = "../../../skill/envault/SKILL.md"

func readSource(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("ler fonte: %v", err)
	}
	return raw
}

func TestSkillContentMatchesSource(t *testing.T) {
	if !bytes.Equal([]byte(Content()), readSource(t)) {
		t.Fatal("content.go fora de sincronia com skill/envault/SKILL.md; rode make skill-sync")
	}
}

func TestSkillContentFrontmatter(t *testing.T) {
	c := Content()
	if !strings.HasPrefix(c, "---\nname: envault\ndescription: ") {
		t.Fatalf("frontmatter inesperado: %q", c[:min(len(c), 60)])
	}
	for _, want := range []string{".env.example", "segredos", "credenciais", "chaves de API"} {
		if !strings.Contains(c, want) {
			t.Errorf("description sem %q", want)
		}
	}
}

func TestSkillContentRules(t *testing.T) {
	c := Content()
	for _, want := range []string{
		"NUNCA rode `envault get`, `envault shell`",
		"NUNCA leia arquivos `.env`",
		"NUNCA rode `envault new`/`edit`",
		"NUNCA rode `envault load` sem confirmação explícita",
		"`envault --version`, `envault list`, `envault show` e `envault plan`",
		"| 3 | env não encontrada |",
		"| 7 | validação |",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("SKILL.md sem %q", want)
		}
	}
}

func TestSkillPath(t *testing.T) {
	got := Path("/tmp/skills")
	want := filepath.Join("/tmp/skills", Name, FileName)
	if got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func TestSkillDefaultDir(t *testing.T) {
	dir, err := DefaultDir("/home/user")
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if dir != filepath.Join("/home/user", ".claude", "skills") {
		t.Fatalf("dir = %q", dir)
	}
}

func TestSkillDefaultDirEmptyHome(t *testing.T) {
	if _, err := DefaultDir(""); err == nil {
		t.Fatal("DefaultDir com home vazio sem erro")
	}
}

func TestSkillPermissionsJSON(t *testing.T) {
	raw, err := PermissionsJSON()
	if err != nil {
		t.Fatalf("PermissionsJSON: %v", err)
	}
	var doc permissionsDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	p := doc.Permissions
	if len(p.Allow) != 4 || len(p.Ask) != 2 || len(p.Deny) != 4 {
		t.Fatalf("permissões = %+v", p)
	}
	for _, want := range []string{"Read(./.env)", "Read(./.env.local)", "Bash(envault get:*)", "Bash(envault shell:*)"} {
		if !slices.Contains(p.Deny, want) {
			t.Errorf("deny sem %q", want)
		}
	}
	if strings.Contains(raw, ".env.example") || strings.Contains(raw, ".env.*") {
		t.Error(".env.example não pode ser bloqueado")
	}
}
