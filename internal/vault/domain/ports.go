package domain

import (
	"context"
	"io"
)

type EnvRepository interface {
	Location() string
	Exists(ctx context.Context) (bool, error)
	Init(ctx context.Context) (bool, error)
	Load(ctx context.Context) (*Snapshot, error)
	Update(ctx context.Context, apply func(*Snapshot) error) error
}

type KeyStore interface {
	Load(ctx context.Context) ([]byte, error)
	Create(ctx context.Context) ([]byte, error)
}

type EditResult struct {
	Env     Env
	Changed bool
	Diff    Diff
}

type Editor interface {
	Edit(ctx context.Context, initial Env, isNew bool) (EditResult, error)
}

type EditorProcess interface {
	Run() error
	SetStdin(r io.Reader)
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
}

type EditorSession interface {
	Process(ctx context.Context) EditorProcess
	Review(runErr error) (EditResult, error)
	Warnings() []string
	Close() error
}

type SessionEditor interface {
	Open(initial Env, isNew bool) (EditorSession, error)
}
