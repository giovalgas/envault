package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func lookupFrom(values map[string]string) Lookup {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func fixedConfigDir(dir string, err error) UserConfigDir {
	return func() (string, error) { return dir, err }
}

func TestLoadDefaultDir(t *testing.T) {
	base := t.TempDir()
	cfg, err := Load(lookupFrom(nil), fixedConfigDir(base, nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(base, AppDirName)
	if cfg.Paths.Dir != want {
		t.Fatalf("Dir = %q, want %q", cfg.Paths.Dir, want)
	}
	if cfg.Paths.Vault != filepath.Join(want, VaultFileName) {
		t.Fatalf("Vault = %q", cfg.Paths.Vault)
	}
	if cfg.Paths.Key != filepath.Join(want, KeyFileName) {
		t.Fatalf("Key = %q", cfg.Paths.Key)
	}
	if cfg.Paths.Lock != filepath.Join(want, LockFileName) {
		t.Fatalf("Lock = %q", cfg.Paths.Lock)
	}
	if cfg.Debug || cfg.KeyOverride != "" || cfg.RuntimeDir != "" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadHomeOverride(t *testing.T) {
	home := filepath.Join(t.TempDir(), "x")
	called := false
	cfg, err := Load(lookupFrom(map[string]string{EnvHome: home}), func() (string, error) {
		called = true
		return "", errors.New("must not be called")
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if called {
		t.Fatal("UserConfigDir consulted despite ENVAULT_HOME")
	}
	if cfg.Paths.Dir != filepath.Clean(home) {
		t.Fatalf("Dir = %q, want %q", cfg.Paths.Dir, home)
	}
}

func TestLoadEmptyHomeFallsBack(t *testing.T) {
	base := t.TempDir()
	cfg, err := Load(lookupFrom(map[string]string{EnvHome: ""}), fixedConfigDir(base, nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Paths.Dir != filepath.Join(base, AppDirName) {
		t.Fatalf("Dir = %q", cfg.Paths.Dir)
	}
}

func TestLoadVariables(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{
		EnvHome:       "/tmp/h",
		EnvKey:        "  abc=  ",
		EnvDebug:      "1",
		EnvRuntimeDir: "/run/user/1000",
	}), fixedConfigDir("", nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.KeyOverride != "abc=" {
		t.Fatalf("KeyOverride = %q", cfg.KeyOverride)
	}
	if !cfg.Debug {
		t.Fatal("Debug = false, want true")
	}
	if cfg.RuntimeDir != "/run/user/1000" {
		t.Fatalf("RuntimeDir = %q", cfg.RuntimeDir)
	}
}

func TestDebugValues(t *testing.T) {
	cases := map[string]bool{
		"1": true, "true": true, "TRUE": true, "yes": true, "on": true,
		"0": false, "false": false, "": false, "nope": false,
	}
	for raw, want := range cases {
		cfg, err := Load(lookupFrom(map[string]string{EnvHome: "/h", EnvDebug: raw}), fixedConfigDir("", nil))
		if err != nil {
			t.Fatalf("Load(%q): %v", raw, err)
		}
		if cfg.Debug != want {
			t.Errorf("ENVAULT_DEBUG=%q: Debug = %v, want %v", raw, cfg.Debug, want)
		}
	}
}

func TestLoadConfigDirErrors(t *testing.T) {
	boom := errors.New("boom")
	if _, err := Load(lookupFrom(nil), fixedConfigDir("", boom)); !errors.Is(err, ErrNoConfigDir) || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want ErrNoConfigDir wrapping boom", err)
	}
	if _, err := Load(lookupFrom(nil), fixedConfigDir("", nil)); !errors.Is(err, ErrNoConfigDir) {
		t.Fatalf("err = %v, want ErrNoConfigDir", err)
	}
}

func TestFromOSUsesEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, home)
	t.Setenv(EnvKey, "")
	t.Setenv(EnvDebug, "true")
	cfg, err := FromOS()
	if err != nil {
		t.Fatalf("FromOS: %v", err)
	}
	if cfg.Paths.Dir != filepath.Clean(home) {
		t.Fatalf("Dir = %q, want %q", cfg.Paths.Dir, home)
	}
	if !cfg.Debug {
		t.Fatal("Debug = false")
	}
	if _, ok := os.LookupEnv(EnvHome); !ok {
		t.Fatal("ENVAULT_HOME not set")
	}
}
