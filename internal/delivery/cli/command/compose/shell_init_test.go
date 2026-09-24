package compose_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/cli"
	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/delivery/cli/clitest"
	"github.com/giovalgas/envault/internal/delivery/cli/presenter"
	"github.com/giovalgas/envault/internal/shared/config"
)

const (
	binaryModeEnv   = "ENVAULT_TEST_AS_BINARY"
	wrapperLogEnv   = "ENVAULT_TEST_LOG"
	wrapperShellEnv = "ENVAULT_TEST_SHELL"
)

const posixWrapperScript = `eval "$(envault shell-init "$ENVAULT_TEST_SHELL")"
envault load a b >"$ENVAULT_TEST_LOG" 2>&1
printf 'load=%s\n' "$?"
printf 'X=%s\n' "$X"
printf 'DB=%s\n' "$DATABASE_URL"
printf 'APP=%s\n' "$APP_URL"
envault load a nao-existe >>"$ENVAULT_TEST_LOG" 2>&1
printf 'missing=%s\n' "$?"
envault show nao-existe >>"$ENVAULT_TEST_LOG" 2>&1
printf 'show=%s\n' "$?"
printf 'passthru=%s\n' "$(envault exec -e a -- sh -c 'printf %s "${ENVAULT_EXPORT_FILE-unset}"')"
`

const fishWrapperScript = `envault shell-init "$ENVAULT_TEST_SHELL" | source
envault load a b >"$ENVAULT_TEST_LOG" 2>&1
printf 'load=%s\n' $status
printf 'X=%s\n' "$X"
printf 'DB=%s\n' "$DATABASE_URL"
printf 'APP=%s\n' "$APP_URL"
envault load a nao-existe >>"$ENVAULT_TEST_LOG" 2>&1
printf 'missing=%s\n' $status
envault show nao-existe >>"$ENVAULT_TEST_LOG" 2>&1
printf 'show=%s\n' $status
printf 'passthru=%s\n' (envault exec -e a -- sh -c 'printf %s "${ENVAULT_EXPORT_FILE-unset}"')
`

func TestMain(m *testing.M) {
	if os.Getenv(binaryModeEnv) == "1" {
		os.Exit(runAsBinary())
	}
	os.Exit(m.Run())
}

func runAsBinary() int {
	return cli.Execute(context.Background(), app.NewApp("test", testWiring(nil)), os.Args[1:])
}

func shellInitSetExport(t *testing.T, file, dialect string) {
	t.Helper()
	t.Setenv(composeusecase.ExportFileVar, file)
	t.Setenv(composeusecase.ExportShellVar, dialect)
}

func TestShellInitPrintsWrapper(t *testing.T) {
	for _, dialect := range composeusecase.ShellDialects() {
		ta := newTestApp(t)
		if code := ta.Run("shell-init", dialect); code != presenter.ExitOK {
			t.Fatalf("%s: code = %d stderr %s", dialect, code, ta.Err.String())
		}
		want, err := composeusecase.NewRenderShellWrapper().Execute(context.Background(), dialect)
		if err != nil {
			t.Fatalf("%s: %v", dialect, err)
		}
		if ta.Out.String() != want || ta.Err.Len() != 0 {
			t.Fatalf("%s: stdout %q stderr %q", dialect, ta.Out.String(), ta.Err.String())
		}
		if !strings.Contains(want, composeusecase.ExportShellVar+"="+dialect) {
			t.Fatalf("%s: wrapper sem dialeto:\n%s", dialect, want)
		}
	}
}

func TestShellInitUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"shell-init"}, {"shell-init", "tcsh"}, {"shell-init", "bash", "zsh"}} {
		ta := newTestApp(t)
		if code := ta.Run(args...); code != presenter.ExitUsage {
			t.Fatalf("%v: code = %d", args, code)
		}
		if ta.Out.Len() != 0 {
			t.Fatalf("%v: stdout = %q", args, ta.Out.String())
		}
	}
}

func TestLoadWithoutWrapperExitsUsage(t *testing.T) {
	dir := planTestWorkdir(t)
	shellInitSetExport(t, "", "")
	t.Setenv("SHELL", "/bin/zsh")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.Run("load", "a", "b"); code != presenter.ExitUsage {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if ta.Out.Len() != 0 {
		t.Fatalf("stdout = %q", ta.Out.String())
	}
	for _, want := range []string{`eval "$(envault shell-init zsh)"`, "--out"} {
		if !strings.Contains(ta.Err.String(), want) {
			t.Fatalf("stderr sem %q: %q", want, ta.Err.String())
		}
	}
	planTestNoSecrets(t, ta)
	for _, value := range []string{"=1", "=2"} {
		if strings.Contains(ta.Err.String(), value) {
			t.Fatalf("stderr vaza %q: %q", value, ta.Err.String())
		}
	}
	if code := ta.Run("load", "a", "--json"); code != presenter.ExitUsage {
		t.Fatalf("json code = %d", code)
	}
	if envelope := clitest.DecodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != presenter.ExitUsage || !strings.Contains(envelope.Error.Message, "shell-init") {
		t.Fatalf("envelope = %+v", envelope)
	}
	planTestNoSecrets(t, ta)
	if entries := planTestEntries(t, dir); len(entries) != 0 {
		t.Fatalf("entries = %v", entries)
	}
}

func TestLoadWithoutWrapperSuggestsFishSyntax(t *testing.T) {
	planTestWorkdir(t)
	shellInitSetExport(t, "", "")
	t.Setenv("SHELL", "/usr/local/bin/fish")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.Run("load", "a"); code != presenter.ExitUsage {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(ta.Err.String(), "envault shell-init fish | source") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
}

func TestLoadShellWritesExportFile(t *testing.T) {
	cases := []struct {
		dialect string
		want    string
	}{
		{dialect: composeusecase.ShellZsh, want: "export X='2'\nexport DATABASE_URL='" + planTestSecretDB + "'\nexport APP_URL='" + planTestSecretApp + "'\n"},
		{dialect: composeusecase.ShellBash, want: "export X='2'\nexport DATABASE_URL='" + planTestSecretDB + "'\nexport APP_URL='" + planTestSecretApp + "'\n"},
		{dialect: composeusecase.ShellFish, want: "set -gx X '2'\nset -gx DATABASE_URL '" + planTestSecretDB + "'\nset -gx APP_URL '" + planTestSecretApp + "'\n"},
	}
	for _, tc := range cases {
		t.Run(tc.dialect, func(t *testing.T) {
			dir := planTestWorkdir(t)
			exports := filepath.Join(t.TempDir(), "exports")
			shellInitSetExport(t, exports, tc.dialect)
			ta := newTestApp(t)
			planTestSeed(t, ta)
			if code := ta.Run("load", "a", "b"); code != presenter.ExitOK {
				t.Fatalf("code = %d stderr %s", code, ta.Err.String())
			}
			if got := planTestRead(t, exports); got != tc.want {
				t.Fatalf("exports = %q", got)
			}
			if ta.Out.Len() != 0 || !strings.Contains(ta.Err.String(), "Exportadas 3 variáveis de a, b no terminal.") || !strings.Contains(ta.Err.String(), "X") {
				t.Fatalf("stdout %q stderr %q", ta.Out.String(), ta.Err.String())
			}
			planTestNoSecrets(t, ta)
			if entries := planTestEntries(t, dir); len(entries) != 0 {
				t.Fatalf("load no terminal gravou arquivos: %v", entries)
			}
			if runtime.GOOS == "windows" {
				return
			}
			info, err := os.Stat(exports)
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}
			if perm := info.Mode().Perm(); perm != 0o600 {
				t.Fatalf("perm = %o", perm)
			}
		})
	}
}

func TestLoadShellDialectFallsBackToShellEnv(t *testing.T) {
	planTestWorkdir(t)
	exports := filepath.Join(t.TempDir(), "exports")
	shellInitSetExport(t, exports, "tcsh")
	t.Setenv("SHELL", "/usr/bin/fish")
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "X", "1"))
	if code := ta.Run("load", "a"); code != presenter.ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, exports); got != "set -gx X '1'\n" {
		t.Fatalf("exports = %q", got)
	}
}

func TestLoadShellJSONAndTemplate(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, composeusecase.DefaultTemplateFile), "PORT=3000\nX=\nSENTRY_DSN=\n")
	exports := filepath.Join(t.TempDir(), "exports")
	shellInitSetExport(t, exports, composeusecase.ShellZsh)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.Run("load", "a", "--only-template", "--json"); code != presenter.ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, exports); got != "export PORT='3000'\nexport X='1'\n" {
		t.Fatalf("exports = %q", got)
	}
	if !strings.Contains(ta.Out.String(), `"target":{"mode":"shell"}`) {
		t.Fatalf("stdout = %s", ta.Out.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if out.Mode != presenter.LoadModeExported || out.Target.Mode != "shell" || out.Target.Path != "" {
		t.Fatalf("out = %+v", out)
	}
	if !strings.Contains(ta.Err.String(), "SENTRY_DSN") || strings.Contains(ta.Err.String(), "Exportadas") {
		t.Fatalf("stderr = %q", ta.Err.String())
	}
	planTestNoSecrets(t, ta)
}

func TestLoadShellErrors(t *testing.T) {
	planTestWorkdir(t)
	shellInitSetExport(t, t.TempDir(), composeusecase.ShellZsh)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.Run("load", "a"); code != presenter.ExitValidation {
		t.Fatalf("directory code = %d", code)
	}
	for _, args := range [][]string{{"load", "a", "--force"}, {"load", "a", "--merge"}, {"load", "a", "--out", ""}} {
		if code := ta.Run(args...); code != presenter.ExitUsage {
			t.Fatalf("%v: code = %d", args, code)
		}
	}
	if code := ta.Run("load", "a", "nao-existe"); code != presenter.ExitEnvNotFound {
		t.Fatalf("not found code = %d", code)
	}
	planTestNoSecrets(t, ta)
}

func TestLoadOutIgnoresWrapper(t *testing.T) {
	dir := planTestWorkdir(t)
	exports := filepath.Join(t.TempDir(), "exports")
	shellInitSetExport(t, exports, composeusecase.ShellZsh)
	ta := newTestApp(t)
	ta.seed(t, planTestEnv("a", "A", "1"))
	if code := ta.Run("load", "a", "--out", ".env"); code != presenter.ExitOK {
		t.Fatalf("code = %d stderr %s", code, ta.Err.String())
	}
	if got := planTestRead(t, filepath.Join(dir, ".env")); got != "A=1\n" {
		t.Fatalf(".env = %q", got)
	}
	if _, err := os.Stat(exports); !os.IsNotExist(err) {
		t.Fatalf("exports não deveria existir: %v", err)
	}
}

func TestPlanWithoutOutReportsShellTarget(t *testing.T) {
	dir := planTestWorkdir(t)
	planTestWrite(t, filepath.Join(dir, ".env"), "LOCAL=1\n")
	shellInitSetExport(t, "", "")
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.Run("plan", "a", "b"); code != presenter.ExitOK {
		t.Fatalf("code = %d stdout %s", code, ta.Out.String())
	}
	if !strings.Contains(ta.Out.String(), `"target":{"mode":"shell"}`) {
		t.Fatalf("stdout = %s", ta.Out.String())
	}
	out := planTestDecode(t, ta.Out.Bytes())
	if out.Target.Mode != "shell" || out.Target.Path != "" || out.Target.Exists {
		t.Fatalf("target = %+v", out.Target)
	}
	if got := out.keyNames(); strings.Join(got, ",") != "X,DATABASE_URL,APP_URL" {
		t.Fatalf("keys = %v", got)
	}
	planTestNoSecrets(t, ta)
	if code := ta.Run("plan", "a", "--out", ""); code != presenter.ExitUsage {
		t.Fatalf("empty out code = %d", code)
	}
	if envelope := clitest.DecodeEnvelope(t, ta.Out.Bytes()); envelope.Error.Code != presenter.ExitUsage {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestPlanOutKeepsFileTargetShape(t *testing.T) {
	planTestWorkdir(t)
	ta := newTestApp(t)
	planTestSeed(t, ta)
	if code := ta.Run("plan", "a", "--out", ".env"); code != presenter.ExitOK {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(ta.Out.String(), `"target":{"path":".env","exists":false,"gitignored":null}`) {
		t.Fatalf("stdout = %s", ta.Out.String())
	}
}

func wrapperCommand(dialect string) *exec.Cmd {
	switch dialect {
	case composeusecase.ShellFish:
		cmd := exec.Command("fish", "--no-config")
		cmd.Stdin = strings.NewReader(fishWrapperScript)
		return cmd
	case composeusecase.ShellZsh:
		cmd := exec.Command("zsh", "-f", "-s")
		cmd.Stdin = strings.NewReader(posixWrapperScript)
		return cmd
	default:
		cmd := exec.Command("bash", "--norc", "--noprofile", "-s")
		cmd.Stdin = strings.NewReader(posixWrapperScript)
		return cmd
	}
}

func wrapperBinDir(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	bin := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(bin, "envault")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	return bin
}

func TestShellWrapperRunsInRealShell(t *testing.T) {
	for _, dialect := range composeusecase.ShellDialects() {
		t.Run(dialect, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("wrapper de shell não roda no Windows")
			}
			if _, err := exec.LookPath(dialect); err != nil {
				t.Skipf("%s não encontrado no PATH", dialect)
			}
			work := planTestWorkdir(t)
			ta := newTestApp(t)
			planTestSeed(t, ta)
			home, tmp, logs := t.TempDir(), t.TempDir(), t.TempDir()
			logFile := filepath.Join(logs, "envault.log")
			cmd := wrapperCommand(dialect)
			cmd.Dir = work
			cmd.Env = []string{
				"PATH=" + wrapperBinDir(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
				"HOME=" + home,
				"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
				"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
				"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
				"TMPDIR=" + tmp,
				config.EnvHome + "=" + ta.Home,
				binaryModeEnv + "=1",
				wrapperShellEnv + "=" + dialect,
				wrapperLogEnv + "=" + logFile,
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: %v\nstdout %s\nstderr %s", dialect, err, stdout.String(), stderr.String())
			}
			want := "load=0\nX=2\nDB=" + planTestSecretDB + "\nAPP=" + planTestSecretApp +
				"\nmissing=3\nshow=3\npassthru=unset\n"
			if stdout.String() != want {
				t.Fatalf("stdout = %q\nwant %q\nstderr %s\nlog %s", stdout.String(), want, stderr.String(), planTestRead(t, logFile))
			}
			log := planTestRead(t, logFile)
			for _, secret := range []string{planTestSecretDB, planTestSecretApp} {
				if strings.Contains(log, secret) || strings.Contains(stderr.String(), secret) {
					t.Fatalf("terminal vaza %q:\nlog %s\nstderr %s", secret, log, stderr.String())
				}
			}
			if !strings.Contains(log, "Exportadas 3 variáveis de a, b no terminal.") {
				t.Fatalf("log = %q", log)
			}
			if entries := planTestEntries(t, tmp); len(entries) != 0 {
				t.Fatalf("temporários do wrapper ficaram: %v", entries)
			}
			if entries := planTestEntries(t, work); len(entries) != 0 {
				t.Fatalf("wrapper gravou no diretório: %v", entries)
			}
		})
	}
}
