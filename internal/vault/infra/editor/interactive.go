package editor

import (
	"context"
	"io"
	"os/exec"
	"slices"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type Interactive struct {
	opts Options
}

var _ domain.SessionEditor = (*Interactive)(nil)

func NewInteractive(opts Options) *Interactive {
	return &Interactive{opts: opts}
}

func (e *Interactive) Open(initial domain.Env, isNew bool) (domain.EditorSession, error) {
	opts := e.opts
	opts.New = isNew
	opts.Stderr = io.Discard
	session, err := Open(initial, opts)
	if err != nil {
		return nil, err
	}
	return interactiveSession{session: session}, nil
}

type interactiveSession struct {
	session *Session
}

func (s interactiveSession) Process(ctx context.Context) domain.EditorProcess {
	return &process{cmd: s.session.Command(ctx)}
}

func (s interactiveSession) Review(runErr error) (domain.EditResult, error) {
	return s.session.Review(runErr)
}

func (s interactiveSession) Warnings() []string {
	return slices.Clone(s.session.Spec().Warnings)
}

func (s interactiveSession) Close() error {
	return s.session.Close()
}

type process struct {
	cmd *exec.Cmd
}

func (p *process) Run() error {
	return p.cmd.Run()
}

func (p *process) SetStdin(r io.Reader) {
	if p.cmd.Stdin == nil {
		p.cmd.Stdin = r
	}
}

func (p *process) SetStdout(w io.Writer) {
	if p.cmd.Stdout == nil {
		p.cmd.Stdout = w
	}
}

func (p *process) SetStderr(w io.Writer) {
	if p.cmd.Stderr == nil {
		p.cmd.Stderr = w
	}
}
