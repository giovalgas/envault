package editor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/giovalgas/envault/internal/vault/domain"
)

func Edit(ctx context.Context, initial domain.Env, opts Options) (Result, error) {
	opts = opts.withDefaults()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	session, err := Open(initial, opts)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()
	for {
		cmd := session.Command(ctx)
		cmd.Stdin = opts.Stdin
		cmd.Stdout = opts.Stdout
		cmd.Stderr = opts.Stderr
		runErr := cmd.Run()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrCanceled, ctxErr)
		}
		result, err := session.Review(runErr)
		if errors.Is(err, ErrReopen) {
			continue
		}
		return result, err
	}
}

type Editor struct {
	opts Options
}

var _ domain.Editor = (*Editor)(nil)

func New(opts Options) *Editor {
	return &Editor{opts: opts}
}

func (e *Editor) Edit(ctx context.Context, initial domain.Env, isNew bool) (domain.EditResult, error) {
	opts := e.opts
	opts.New = isNew
	return Edit(ctx, initial, opts)
}
