package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/giovalgas/envault/internal/vault/domain"
)

type fakeProcess struct {
	err error
}

func (p fakeProcess) Run() error        { return p.err }
func (fakeProcess) SetStdin(io.Reader)  {}
func (fakeProcess) SetStdout(io.Writer) {}
func (fakeProcess) SetStderr(io.Writer) {}

type fakeSession struct {
	initial  domain.Env
	reviews  []func(initial domain.Env) (domain.EditResult, error)
	reviewed int
	closed   int
	closeErr error
}

func (s *fakeSession) Process(context.Context) domain.EditorProcess {
	return fakeProcess{}
}

func (s *fakeSession) Review(error) (domain.EditResult, error) {
	step := s.reviews[s.reviewed]
	s.reviewed++
	return step(s.initial)
}

func (s *fakeSession) Warnings() []string {
	return []string{"aviso"}
}

func (s *fakeSession) Close() error {
	s.closed++
	return s.closeErr
}

type fakeSessionEditor struct {
	session *fakeSession
	isNew   bool
	opened  int
	openErr error
}

func (e *fakeSessionEditor) Open(initial domain.Env, isNew bool) (domain.EditorSession, error) {
	e.opened++
	e.isNew = isNew
	if e.openErr != nil {
		return nil, e.openErr
	}
	e.session.initial = initial
	return e.session, nil
}

func sessionTo(steps ...func(domain.Env) (domain.EditResult, error)) *fakeSessionEditor {
	return &fakeSessionEditor{session: &fakeSession{reviews: steps}}
}

func reviewTo(content domain.Env) func(domain.Env) (domain.EditResult, error) {
	editor := editTo(content)
	return editor.result
}

func reviewErr(err error) func(domain.Env) (domain.EditResult, error) {
	return func(domain.Env) (domain.EditResult, error) { return domain.EditResult{}, err }
}

func TestBeginEditEnvReopensThenApplies(t *testing.T) {
	repo := newRepo(sampleEnv())
	editor := sessionTo(reviewErr(domain.ErrEditReopen), reviewTo(domain.Env{Description: "nova", Vars: []domain.Var{{Key: "POOL", Value: "20"}}}))
	draft, err := NewBeginEditEnv(repo, editor, later).Execute(context.Background(), "a")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if draft.Name() != "a" || draft.IsNew() || editor.isNew || strings.Join(draft.Warnings(), ",") != "aviso" {
		t.Fatalf("draft = %+v", draft)
	}
	if err := draft.Process(context.Background()).Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := draft.Review(nil); !errors.Is(err, domain.ErrEditReopen) || editor.session.closed != 0 {
		t.Fatalf("reopen err = %v, closed %d", err, editor.session.closed)
	}
	result, err := draft.Review(nil)
	if err != nil || !result.Changed || editor.session.closed != 1 {
		t.Fatalf("review = %+v, %v, closed %d", result, err, editor.session.closed)
	}
	if repo.updates != 0 {
		t.Fatal("review must not write before Apply")
	}
	env, err := draft.Apply(context.Background(), result)
	if err != nil || env.Description != "nova" || strings.Join(env.Keys(), ",") != "POOL" {
		t.Fatalf("Apply = %+v, %v", env, err)
	}
	if stored := repo.env(t, "a"); stored.Description != "nova" || !stored.UpdatedAt.Equal(later.Now()) {
		t.Fatalf("stored = %+v", stored)
	}
	if _, err := draft.Review(nil); !errors.Is(err, errDraftFinished) {
		t.Fatalf("review after finish err = %v", err)
	}
}

func TestBeginEditEnvDetectsConcurrentChange(t *testing.T) {
	repo := newRepo(sampleEnv())
	editor := sessionTo(reviewTo(domain.Env{Vars: []domain.Var{{Key: "Z", Value: "z"}}}))
	draft, err := NewBeginEditEnv(repo, editor, later).Execute(context.Background(), "a")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result, err := draft.Review(nil)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if _, err := repo.snapshot.Modify("a", baseTime, func(env *domain.Env) error {
		env.Set("X", "1")
		return nil
	}); err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if _, err := draft.Apply(context.Background(), result); !errors.Is(err, domain.ErrEditConflict) {
		t.Fatalf("err = %v, want ErrEditConflict", err)
	}
	if _, ok := repo.env(t, "a").Lookup("X"); !ok {
		t.Fatal("concurrent change lost")
	}
}

func TestBeginEditEnvUnchangedAndErrors(t *testing.T) {
	repo := newRepo(sampleEnv())
	editor := sessionTo(reviewTo(sampleEnv()))
	draft, err := NewBeginEditEnv(repo, editor, later).Execute(context.Background(), "a")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result, err := draft.Review(nil)
	if err != nil || result.Changed {
		t.Fatalf("review = %+v, %v", result, err)
	}
	if _, err := draft.Apply(context.Background(), result); err != nil || repo.updates != 0 {
		t.Fatalf("Apply err = %v, updates %d", err, repo.updates)
	}
	if _, err := NewBeginEditEnv(repo, editor, later).Execute(context.Background(), "b"); !errors.Is(err, domain.ErrEnvNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	boom := errors.New("boom")
	failing := &fakeSessionEditor{openErr: boom}
	if _, err := NewBeginEditEnv(repo, failing, later).Execute(context.Background(), "a"); !errors.Is(err, boom) {
		t.Fatalf("open err = %v", err)
	}
}

func TestBeginEditEnvCancelClosesSession(t *testing.T) {
	editor := sessionTo(reviewErr(domain.ErrEditCanceled))
	draft, err := NewBeginEditEnv(newRepo(sampleEnv()), editor, later).Execute(context.Background(), "a")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := draft.Review(nil); !errors.Is(err, domain.ErrEditCanceled) || editor.session.closed != 1 {
		t.Fatalf("err = %v, closed %d", err, editor.session.closed)
	}
}

func TestBeginEditEnvReportsCloseFailure(t *testing.T) {
	editor := sessionTo(reviewTo(sampleEnv()))
	editor.session.closeErr = errors.New("disco")
	draft, err := NewBeginEditEnv(newRepo(sampleEnv()), editor, later).Execute(context.Background(), "a")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := draft.Review(nil); err == nil || !strings.Contains(err.Error(), "temporário") {
		t.Fatalf("err = %v", err)
	}
}

func TestBeginCreateEnv(t *testing.T) {
	repo := newRepo(sampleEnv())
	editor := sessionTo(reviewTo(domain.Env{Description: "do editor", Vars: []domain.Var{{Key: "A", Value: "1"}}}))
	draft, err := NewBeginCreateEnv(repo, editor, later).Execute(context.Background(), domain.Env{Name: "nova"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !draft.IsNew() || !editor.isNew || draft.Name() != "nova" {
		t.Fatalf("draft = %+v", draft)
	}
	result, err := draft.Review(nil)
	if err != nil || !result.Changed {
		t.Fatalf("review = %+v, %v", result, err)
	}
	created, err := draft.Apply(context.Background(), result)
	if err != nil || created.Name != "nova" || !created.CreatedAt.Equal(later.Now()) {
		t.Fatalf("Apply = %+v, %v", created, err)
	}
	if stored := repo.env(t, "nova"); stored.Description != "do editor" {
		t.Fatalf("stored = %+v", stored)
	}
	if err := draft.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBeginCreateEnvRejects(t *testing.T) {
	repo := newRepo(sampleEnv())
	editor := sessionTo()
	if _, err := NewBeginCreateEnv(repo, editor, later).Execute(context.Background(), domain.Env{Name: "a"}); !errors.Is(err, domain.ErrEnvExists) {
		t.Fatalf("exists err = %v", err)
	}
	if _, err := NewBeginCreateEnv(repo, editor, later).Execute(context.Background(), domain.Env{Name: "../x"}); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("invalid err = %v", err)
	}
	if _, err := NewBeginCreateEnv(&memRepo{}, editor, later).Execute(context.Background(), domain.Env{Name: "b"}); !errors.Is(err, domain.ErrNotInitialized) {
		t.Fatalf("not initialized err = %v", err)
	}
	if editor.opened != 0 {
		t.Fatalf("editor opened %d times", editor.opened)
	}
	boom := errors.New("boom")
	if _, err := NewBeginCreateEnv(repo, &fakeSessionEditor{openErr: boom}, later).Execute(context.Background(), domain.Env{Name: "b"}); !errors.Is(err, boom) {
		t.Fatalf("open err = %v", err)
	}
}

func TestBeginCreateEnvDetectsConcurrentCreate(t *testing.T) {
	repo := newRepo()
	editor := sessionTo(reviewTo(domain.Env{Vars: []domain.Var{{Key: "A", Value: "1"}}}))
	draft, err := NewBeginCreateEnv(repo, editor, later).Execute(context.Background(), domain.Env{Name: "nova"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result, err := draft.Review(nil)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if _, err := repo.snapshot.Create(domain.Env{Name: "nova"}, baseTime); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := draft.Apply(context.Background(), result); !errors.Is(err, domain.ErrEnvExists) {
		t.Fatalf("err = %v, want ErrEnvExists", err)
	}
}
