package clitest

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/giovalgas/envault/internal/delivery/cli/app"
	"github.com/giovalgas/envault/internal/shared/config"
)

type Executor func(ctx context.Context, a *app.App, args []string) int

type Wire func(h *Harness) app.Wiring

type Harness struct {
	App      *app.App
	Home     string
	In       *bytes.Buffer
	Out      *bytes.Buffer
	Err      *bytes.Buffer
	Keychain bool

	Clipboard    []string
	ClipboardErr error

	execute   Executor
	stdinTTY  bool
	stdoutTTY bool
}

type ErrorEnvelope struct {
	SchemaVersion int `json:"schema_version"`
	Error         struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func New(t *testing.T, execute Executor, wire Wire) *Harness {
	t.Helper()
	home := filepath.Join(t.TempDir(), "envault")
	t.Setenv(config.EnvHome, home)
	t.Setenv(config.EnvKey, "")
	t.Setenv(config.EnvDebug, "")
	h := &Harness{
		Home:    home,
		In:      &bytes.Buffer{},
		Out:     &bytes.Buffer{},
		Err:     &bytes.Buffer{},
		execute: execute,
	}
	h.App = &app.App{
		Stdin:      h.In,
		Stdout:     h.Out,
		Stderr:     h.Err,
		Version:    "test",
		LoadConfig: config.FromOS,
		IsTerminal: h.IsTerminal,
		Clipboard:  h.WriteClipboard,
	}
	h.App.Wire = wire(h)
	return h
}

func (h *Harness) Run(args ...string) int {
	h.Out.Reset()
	h.Err.Reset()
	return h.execute(context.Background(), h.App, args)
}

func (h *Harness) IsTerminal(stream any) bool {
	switch stream {
	case any(h.In):
		return h.stdinTTY
	case any(h.Out):
		return h.stdoutTTY
	default:
		return false
	}
}

func (h *Harness) WriteClipboard(text string) error {
	if h.ClipboardErr != nil {
		return h.ClipboardErr
	}
	h.Clipboard = append(h.Clipboard, text)
	return nil
}

func (h *Harness) SetTerminal(stdin, stdout bool) {
	h.stdinTTY = stdin
	h.stdoutTTY = stdout
}

func (h *Harness) UseKeychain() {
	h.Keychain = true
}

func (h *Harness) Streams() app.Streams {
	return app.Streams{Stdin: h.In, Stdout: h.Out, Stderr: h.Err}
}

func (h *Harness) Config(t *testing.T) config.Config {
	t.Helper()
	cfg, err := h.App.Config()
	if err != nil {
		t.Fatalf("Config: %v", err)
	}
	return cfg
}

func DecodeEnvelope(t *testing.T, data []byte) ErrorEnvelope {
	t.Helper()
	var envelope ErrorEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, data)
	}
	return envelope
}
