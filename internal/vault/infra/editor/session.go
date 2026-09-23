package editor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/giovalgas/envault/internal/shared/dotenv"
	"github.com/giovalgas/envault/internal/vault/domain"
)

const (
	errorHeaderPrefix = "# ERRO "
	stopGracePeriod   = 2 * time.Second
)

var (
	ErrCanceled = domain.ErrEditCanceled
	ErrReopen   = domain.ErrEditReopen
	ErrEditor   = errors.New("o editor terminou com erro")
)

type Options struct {
	New        bool
	Lookup     LookupFunc
	GOOS       string
	RuntimeDir string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

func (o Options) withDefaults() Options {
	if o.Lookup == nil {
		o.Lookup = os.LookupEnv
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	return o
}

type (
	Result = domain.EditResult
	Diff   = domain.Diff
)

type Session struct {
	initial  domain.Env
	opts     Options
	spec     Spec
	dir      string
	path     string
	original [sha256.Size]byte
	once     sync.Once
	closeErr error
}

func Open(initial domain.Env, opts Options) (*Session, error) {
	opts = opts.withDefaults()
	spec, err := Resolve(opts.Lookup, opts.GOOS)
	if err != nil {
		return nil, err
	}
	dir, err := makePrivateDir(opts.RuntimeDir, opts.GOOS)
	if err != nil {
		return nil, err
	}
	s := &Session{initial: initial.Clone(), opts: opts, spec: spec, dir: dir, path: tempPath(dir, initial)}
	content := dotenv.FormatEditor(initial.Document())
	s.original = sha256.Sum256(content)
	if err := writePrivate(s.path, content, opts.GOOS); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("gravar arquivo temporário: %w", err)
	}
	if err := writeWarnings(opts.Stderr, spec.Warnings); err != nil {
		return nil, errors.Join(fmt.Errorf("exibir avisos do editor: %w", err), s.Close())
	}
	return s, nil
}

func writeWarnings(w io.Writer, warnings []string) error {
	for _, warning := range warnings {
		if _, err := fmt.Fprintln(w, warning); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) Path() string {
	return s.path
}

func (s *Session) Spec() Spec {
	return s.spec
}

func (s *Session) Command(ctx context.Context) *exec.Cmd {
	argv := s.spec.Argv(s.path)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error {
		if s.opts.GOOS == goosWindows {
			return cmd.Process.Kill()
		}
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = stopGracePeriod
	return cmd
}

func (s *Session) Review(runErr error) (Result, error) {
	if runErr != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrEditor, runErr)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return Result{}, fmt.Errorf("ler arquivo temporário: %w", err)
	}
	if s.canceledBy(data) {
		return Result{}, ErrCanceled
	}
	if sha256.Sum256(data) == s.original {
		return Result{Env: s.initial.Clone()}, nil
	}
	parsed, err := dotenv.Parse(data)
	if err != nil {
		return Result{}, s.reopen(data, err)
	}
	content := domain.FromDocument(parsed)
	edited := s.initial.Clone()
	edited.Description = content.Description
	edited.Tags = content.Tags
	edited.Vars = content.Vars
	if err := edited.ValidateContent(); err != nil {
		return Result{}, s.reopen(data, err)
	}
	if edited.SameContent(s.initial) {
		return Result{Env: s.initial.Clone()}, nil
	}
	return Result{Env: edited, Changed: true, Diff: domain.Compare(s.initial, edited)}, nil
}

func (s *Session) Close() error {
	s.once.Do(func() {
		s.closeErr = os.RemoveAll(s.dir)
	})
	return s.closeErr
}

func (s *Session) canceledBy(data []byte) bool {
	if s.opts.New {
		return dotenv.IsBlank(data)
	}
	return len(bytes.TrimSpace(data)) == 0
}

func (s *Session) reopen(data []byte, cause error) error {
	content := errorHeaderPrefix + errorMessage(cause) + "\n" + stripErrorHeaders(string(data))
	if err := writePrivate(s.path, []byte(content), s.opts.GOOS); err != nil {
		return fmt.Errorf("regravar arquivo temporário: %w", err)
	}
	return fmt.Errorf("%w: %w", ErrReopen, cause)
}

func errorMessage(cause error) string {
	var parseErr *dotenv.ParseError
	if errors.As(cause, &parseErr) {
		return parseErr.Error()
	}
	return strings.ReplaceAll(cause.Error(), "\n", " ")
}

func stripErrorHeaders(content string) string {
	for strings.HasPrefix(content, errorHeaderPrefix) {
		_, rest, found := strings.Cut(content, "\n")
		if !found {
			return ""
		}
		content = rest
	}
	return content
}
