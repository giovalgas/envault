package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/shared/dotenv"
)

func loadTestParsed(t *testing.T, path string) []string {
	t.Helper()
	env, err := dotenv.Parse([]byte(planTestRead(t, path)))
	if err != nil {
		t.Fatalf("Parse(%s): %v", path, err)
	}
	pairs := make([]string, len(env.Vars))
	for i, v := range env.Vars {
		pairs[i] = v.Key + "=" + v.Value
	}
	return pairs
}

func TestLoadCreatesTarget(t *testing.T) {
	dir := planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "b"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	want := []string{"X=2", "DATABASE_URL=" + planTestSecretDB, "APP_URL=" + planTestSecretApp}
	if got := loadTestParsed(t, filepath.Join(dir, ".env")); !slices.Equal(got, want) {
		t.Fatalf(".env = %v", got)
	}
	if ta.Out.Len() != 0 {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
	if !strings.Contains(ta.Err.String(), "Gravado .env com 3 variáveis de a, b") || !strings.Contains(ta.Err.String(), "X") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	planTestNoSecrets(t, ta)
	if entries := planTestEntries(t, dir); !slices.Equal(entries, []string{".env"}) {
		t.Fatalf("entries = %v", entries)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, ".env"))
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("perm = %o", perm)
		}
	}
}

func TestLoadRefusesExistingTarget(t *testing.T) {
	dir := planTestWorkdir(t)
	target := filepath.Join(dir, ".env")
	planTestWrite(t, target, "LOCAL=1\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--out", ".env"); code != ExitTargetExists {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if !strings.Contains(ta.Err.String(), "--force") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--json"); code != ExitTargetExists {
		t.Fatalf("json code = %d", code)
	}
	if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != ExitTargetExists {
		t.Fatalf("envelope = %+v", envelope)
	}
	if got := planTestRead(t, target); got != "LOCAL=1\n" {
		t.Fatalf(".env changed: %q", got)
	}
	if entries := planTestEntries(t, dir); !slices.Equal(entries, []string{".env"}) {
		t.Fatalf("entries = %v", entries)
	}
}

func TestLoadForceReplaces(t *testing.T) {
	dir := planTestWorkdir(t)
	target := filepath.Join(dir, ".env")
	planTestWrite(t, target, "LOCAL=1\nX=0\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "b", "--force"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := loadTestParsed(t, target); !slices.Equal(got, []string{"X=2", "APP_URL=" + planTestSecretApp}) {
		t.Fatalf(".env = %v", got)
	}
	if !strings.Contains(ta.Err.String(), "Substituído") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestLoadMergePreservesFileOrder(t *testing.T) {
	dir := planTestWorkdir(t)
	target := filepath.Join(dir, ".env")
	planTestWrite(t, target, "A=old\nLOCAL=1\n")
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "new", "B", "2"))
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--merge"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, target); got != "A=new\nLOCAL=1\nB=2\n" {
		t.Fatalf(".env = %q", got)
	}
	if !strings.Contains(ta.Err.String(), "Mesclado") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestLoadMergeWithoutTargetCreates(t *testing.T) {
	dir := planTestWorkdir(t)
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "1"))
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--merge", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, filepath.Join(dir, ".env")); got != "A=1\n" {
		t.Fatalf(".env = %q", got)
	}
	if out := planTestDecode(t, ta.Out.Bytes()); out.Mode != loadModeCreated || out.Target.Exists {
		t.Fatalf("out = %+v", out)
	}
}

func TestLoadMergeInvalidTargetKeepsFile(t *testing.T) {
	dir := planTestWorkdir(t)
	target := filepath.Join(dir, ".env")
	planTestWrite(t, target, "NOT VALID\n")
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "1"))
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--merge"); code != ExitValidation {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, target); got != "NOT VALID\n" {
		t.Fatalf(".env = %q", got)
	}
}

func TestLoadForceAndMergeIsUsageError(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--force", "--merge"); code != ExitUsage {
		t.Fatalf("code = %d", code)
	}
}

func TestLoadTemplate(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, templateDefaultPath), "PORT=3000\nX=\nSENTRY_DSN=\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--only-template"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, filepath.Join(dir, ".env")); got != "PORT=3000\nX=1\n" {
		t.Fatalf(".env = %q", got)
	}
	if !strings.Contains(ta.Err.String(), "SENTRY_DSN") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestLoadGitignoreWarning(t *testing.T) {
	dir := planTestRepo(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--out", ".env", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if !strings.Contains(ta.Err.String(), "aviso") || !strings.Contains(ta.Err.String(), ".gitignore") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if out.Target.Gitignored == nil || *out.Target.Gitignored || out.Mode != loadModeCreated {
		t.Fatalf("out = %+v", out)
	}
	if got := out.keyNames(); !slices.Equal(got, []string{"X", "DATABASE_URL"}) {
		t.Fatalf("keys = %v", got)
	}
	planTestNoSecrets(t, ta)
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore touched: %v", err)
	}
}

func TestLoadGitignoreCovered(t *testing.T) {
	dir := planTestRepo(t)
	planTestWrite(t, filepath.Join(dir, ".gitignore"), ".env\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--json"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if ta.Err.Len() != 0 {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if out.Target.Gitignored == nil || !*out.Target.Gitignored {
		t.Fatalf("target = %+v", out.Target)
	}
	if got := planTestRead(t, filepath.Join(dir, ".gitignore")); got != ".env\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

func TestLoadOutsideRepoHasNoWarning(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--json"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	if ta.Err.Len() != 0 {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	if out := planTestDecode(t, ta.Out.Bytes()); out.Target.Gitignored != nil {
		t.Fatalf("target = %+v", out.Target)
	}
}

func TestLoadOutSubdirectory(t *testing.T) {
	dir := planTestWorkdir(t)
	if err := os.Mkdir(filepath.Join(dir, "config"), 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "1"))
	out := filepath.Join("config", "app.env")
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--out", out); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, filepath.Join(dir, out)); got != "A=1\n" {
		t.Fatalf("%s = %q", out, got)
	}
	if entries := planTestEntries(t, filepath.Join(dir, "config")); !slices.Equal(entries, []string{"app.env"}) {
		t.Fatalf("entries = %v", entries)
	}
}

func TestLoadForceOnDirectoryFails(t *testing.T) {
	dir := planTestWorkdir(t)
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0o700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "1"))
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--force"); code != ExitValidation {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
}

func TestLoadForceFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink exige privilégio no Windows")
	}
	dir := planTestWorkdir(t)
	realPath := filepath.Join(dir, "real.env")
	planTestWrite(t, realPath, "OLD=1\n")
	if err := os.Symlink("real.env", filepath.Join(dir, ".env")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "1"))
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "--force"); code != ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	info, err := os.Lstat(filepath.Join(dir, ".env"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v %v", info, err)
	}
	if got := planTestRead(t, realPath); got != "A=1\n" {
		t.Fatalf("real.env = %q", got)
	}
}

func TestLoadEnvNotFound(t *testing.T) {
	dir := planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newLoadCmd(ta.App), "load", "a", "nao-existe"); code != ExitEnvNotFound {
		t.Fatalf("code = %d", code)
	}
	if entries := planTestEntries(t, dir); len(entries) != 0 {
		t.Fatalf("entries = %v", entries)
	}
}
