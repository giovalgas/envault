package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault"
)

const (
	planTestSecretDB  = "postgres://usuario:senhaforte@db/app"
	planTestSecretApp = "supersegredo"
)

type planTestKey struct {
	Key     string   `json:"key"`
	From    *string  `json:"from"`
	Shadows []string `json:"shadows"`
	Default bool     `json:"default"`
}

type planTestOutput struct {
	SchemaVersion int      `json:"schema_version"`
	Envs          []string `json:"envs"`
	Target        struct {
		Path       string `json:"path"`
		Exists     bool   `json:"exists"`
		Gitignored *bool  `json:"gitignored"`
	} `json:"target"`
	Template struct {
		Path  *string `json:"path"`
		Found bool    `json:"found"`
	} `json:"template"`
	Keys      []planTestKey `json:"keys"`
	Conflicts []string      `json:"conflicts"`
	Missing   []string      `json:"missing"`
	Extra     []string      `json:"extra"`
	Mode      string        `json:"mode"`
}

func (o planTestOutput) keyNames() []string {
	names := make([]string, len(o.Keys))
	for i, k := range o.Keys {
		names[i] = k.Key
	}
	return names
}

func (o planTestOutput) key(t *testing.T, name string) planTestKey {
	t.Helper()
	for _, k := range o.Keys {
		if k.Key == name {
			return k
		}
	}
	t.Fatalf("key %s not in %v", name, o.keyNames())
	return planTestKey{}
}

func planTestDecode(t *testing.T, data []byte) planTestOutput {
	t.Helper()
	var out planTestOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, data)
	}
	for _, field := range []string{`"conflicts":[`, `"missing":[`, `"extra":[`, `"keys":[`, `"envs":[`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("stdout lacks %s: %s", field, data)
		}
	}
	return out
}

func planTestChdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})
}

func planTestGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func planTestWorkdir(t *testing.T) string {
	t.Helper()
	planTestGitConfig(t)
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	planTestChdir(t, dir)
	return dir
}

func planTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git indisponível")
	}
	dir := planTestWorkdir(t)
	if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func planTestWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func planTestRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(data)
}

func planTestEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func planTestEnv(name string, pairs ...string) vault.Env {
	env := vault.Env{Name: name}
	for i := 0; i+1 < len(pairs); i += 2 {
		env.Vars = append(env.Vars, vault.Var{Key: pairs[i], Value: pairs[i+1]})
	}
	return env
}

func planTestSeed(t *testing.T, ta *testApp) {
	t.Helper()
	ta.seed(t,
		planTestEnv("a", "X", "1", "DATABASE_URL", planTestSecretDB),
		planTestEnv("b", "X", "2", "APP_URL", planTestSecretApp),
	)
}

func planTestNoSecrets(t *testing.T, ta *testApp) {
	t.Helper()
	for _, secret := range []string{planTestSecretDB, planTestSecretApp} {
		if strings.Contains(ta.Out.String(), secret) || strings.Contains(ta.Err.String(), secret) {
			t.Fatalf("output leaks %q:\nstdout %s\nstderr %s", secret, ta.Out.String(), ta.Err.String())
		}
	}
}

func TestPlanDoesNotWriteAndReportsPlan(t *testing.T) {
	dir := planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "b", "--out", ".env"); code != ExitOK {
		t.Fatalf("code = %d stderr %s stdout %s", code, ta.Err.String(), ta.Out.String())
	}
	if entries := planTestEntries(t, dir); len(entries) != 0 {
		t.Fatalf("plan wrote files: %v", entries)
	}
	planTestNoSecrets(t, ta)
	if ta.Err.Len() != 0 {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if out.SchemaVersion != SchemaVersion || !slices.Equal(out.Envs, []string{"a", "b"}) {
		t.Fatalf("header = %+v", out)
	}
	if out.Target.Path != ".env" || out.Target.Exists || out.Target.Gitignored != nil {
		t.Fatalf("target = %+v", out.Target)
	}
	if out.Template.Found || out.Template.Path == nil || *out.Template.Path != templateDefaultPath {
		t.Fatalf("template = %+v", out.Template)
	}
	if got := out.keyNames(); !slices.Equal(got, []string{"X", "DATABASE_URL", "APP_URL"}) {
		t.Fatalf("keys = %v", got)
	}
	x := out.key(t, "X")
	if x.From == nil || *x.From != "b" || !slices.Equal(x.Shadows, []string{"a"}) || x.Default {
		t.Fatalf("X = %+v", x)
	}
	db := out.key(t, "DATABASE_URL")
	if db.From == nil || *db.From != "a" || db.Shadows == nil || len(db.Shadows) != 0 {
		t.Fatalf("DATABASE_URL = %+v", db)
	}
	if !slices.Equal(out.Conflicts, []string{"X"}) || len(out.Missing) != 0 || len(out.Extra) != 0 {
		t.Fatalf("conflicts %v missing %v extra %v", out.Conflicts, out.Missing, out.Extra)
	}
}

func TestPlanTemplateDetection(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, templateDefaultPath), "PORT=3000\nX=\nSENTRY_DSN=\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a"); code != ExitOK {
		t.Fatalf("code = %d stdout %s", code, ta.Out.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if !out.Template.Found || *out.Template.Path != templateDefaultPath {
		t.Fatalf("template = %+v", out.Template)
	}
	if got := out.keyNames(); !slices.Equal(got, []string{"PORT", "X", "DATABASE_URL"}) {
		t.Fatalf("keys = %v", got)
	}
	port := out.key(t, "PORT")
	if !port.Default || port.From != nil {
		t.Fatalf("PORT = %+v", port)
	}
	if !slices.Equal(out.Missing, []string{"SENTRY_DSN"}) || !slices.Equal(out.Extra, []string{"DATABASE_URL"}) {
		t.Fatalf("missing %v extra %v", out.Missing, out.Extra)
	}
	if entries := planTestEntries(t, dir); !slices.Equal(entries, []string{templateDefaultPath}) {
		t.Fatalf("entries = %v", entries)
	}
}

func TestPlanNoTemplate(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, templateDefaultPath), "SENTRY_DSN=\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "--no-template"); code != ExitOK {
		t.Fatalf("code = %d stdout %s", code, ta.Out.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if out.Template.Found || out.Template.Path != nil {
		t.Fatalf("template = %+v", out.Template)
	}
	if len(out.Missing) != 0 || len(out.Extra) != 0 {
		t.Fatalf("missing %v extra %v", out.Missing, out.Extra)
	}
}

func TestPlanExplicitTemplateAndOnlyTemplate(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, "custom.example"), "APP_URL=\nX=\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "b", "--template", "custom.example", "--only-template")
	if code != ExitOK {
		t.Fatalf("code = %d stdout %s", code, ta.Out.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if !out.Template.Found || *out.Template.Path != "custom.example" {
		t.Fatalf("template = %+v", out.Template)
	}
	if got := out.keyNames(); !slices.Equal(got, []string{"APP_URL", "X"}) {
		t.Fatalf("keys = %v", got)
	}
	if !slices.Equal(out.Extra, []string{"DATABASE_URL"}) {
		t.Fatalf("extra = %v", out.Extra)
	}
	planTestNoSecrets(t, ta)
}

func TestPlanTemplateErrors(t *testing.T) {
	cases := []struct {
		name     string
		template string
		args     []string
		code     int
	}{
		{name: "explicit missing", args: []string{"--template", "nao-existe.example"}, code: ExitValidation},
		{name: "syntax error", template: "NOT VALID\n", args: nil, code: ExitValidation},
		{name: "only template without template", args: []string{"--only-template"}, code: ExitValidation},
		{name: "template with no-template", args: []string{"--template", "x", "--no-template"}, code: ExitUsage},
		{name: "only with no-template", args: []string{"--only-template", "--no-template"}, code: ExitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := planTestWorkdir(t)
			if tc.template != "" {
				planTestWrite(t, filepath.Join(dir, templateDefaultPath), tc.template)
			}
			ta := newTestApp(t)
			planTestSeed(t, ta)
			args := append([]string{"plan", "a"}, tc.args...)
			if code := ta.runWith(newPlanCmd(ta.App), args...); code != tc.code {
				t.Fatalf("code = %d, want %d (stdout %s)", code, tc.code, ta.Out.String())
			}
			if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != tc.code {
				t.Fatalf("envelope = %+v", envelope)
			}
		})
	}
}

func TestPlanVaultErrorsAreJSON(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a"); code != ExitNotInitialized {
		t.Fatalf("code = %d", code)
	}
	if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != ExitNotInitialized {
		t.Fatalf("envelope = %+v", envelope)
	}
	planTestSeed(t, ta)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "nao-existe"); code != ExitEnvNotFound {
		t.Fatalf("code = %d", code)
	}
	if envelope := decodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != ExitEnvNotFound {
		t.Fatalf("envelope = %+v", envelope)
	}
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "a"); code != ExitUsage {
		t.Fatalf("duplicate env code = %d", code)
	}
	if code := ta.runWith(newPlanCmd(ta.App), "plan"); code != ExitUsage {
		t.Fatalf("no args code = %d", code)
	}
}

func TestPlanAcceptsJSONFlag(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "--json"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	planTestDecode(t, ta.Out.Bytes())
}

func TestPlanTargetInRepo(t *testing.T) {
	dir := planTestRepo(t)
	planTestWrite(t, filepath.Join(dir, ".gitignore"), ".env\n")
	planTestWrite(t, filepath.Join(dir, ".env"), "LOCAL=1\n")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a"); code != ExitOK {
		t.Fatalf("code = %d stdout %s", code, ta.Out.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if !out.Target.Exists || out.Target.Gitignored == nil || !*out.Target.Gitignored {
		t.Fatalf("target = %+v", out.Target)
	}
	if code := ta.runWith(newPlanCmd(ta.App), "plan", "a", "--out", "exposto.env"); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
	out = planTestDecode(t, ta.Out.Bytes())
	if out.Target.Exists || out.Target.Gitignored == nil || *out.Target.Gitignored {
		t.Fatalf("target = %+v", out.Target)
	}
	if got := planTestRead(t, filepath.Join(dir, ".env")); got != "LOCAL=1\n" {
		t.Fatalf(".env changed: %q", got)
	}
}
