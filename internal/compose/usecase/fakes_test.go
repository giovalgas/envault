package usecase

import (
	"context"
	"errors"

	"github.com/giovalgas/envault/internal/compose/domain"
)

var errBoom = errors.New("boom")

type fakeReader struct {
	envs  map[string]domain.Env
	err   error
	calls int
}

func newReader(envs ...domain.Env) *fakeReader {
	r := &fakeReader{envs: map[string]domain.Env{}}
	for _, env := range envs {
		r.envs[env.Name] = env
	}
	return r
}

func (r *fakeReader) ReadEnvs(_ context.Context, names []string) ([]domain.Env, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	out := make([]domain.Env, 0, len(names))
	for _, name := range names {
		env, ok := r.envs[name]
		if !ok {
			return nil, &EnvNotFoundError{Name: name}
		}
		out = append(out, env)
	}
	return out, nil
}

type fakeFiles struct {
	exists    bool
	existsErr error
	writeErr  error
	mergeErr  error
	written   []domain.Var
	merged    []domain.Var
	mergeSize int
	writes    int
	merges    int
}

func (f *fakeFiles) Exists(string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeFiles) Write(_ string, vars []domain.Var) error {
	f.writes++
	f.written = vars
	return f.writeErr
}

func (f *fakeFiles) Merge(_ string, vars []domain.Var) (int, error) {
	f.merges++
	f.merged = vars
	return f.mergeSize, f.mergeErr
}

type fakeGitignore struct {
	status domain.GitignoreStatus
	paths  []string
}

func (g *fakeGitignore) IsIgnored(path string) domain.GitignoreStatus {
	g.paths = append(g.paths, path)
	return g.status
}

func env(name string, pairs ...string) domain.Env {
	e := domain.Env{Name: name}
	for i := 0; i+1 < len(pairs); i += 2 {
		e.Vars = append(e.Vars, domain.Var{Key: pairs[i], Value: pairs[i+1]})
	}
	return e
}

func vars(pairs ...string) []domain.Var {
	return env("", pairs...).Vars
}

type fakeExports struct {
	err    error
	path   string
	script string
	writes int
}

func (f *fakeExports) WriteExports(path, script string) error {
	f.writes++
	f.path = path
	f.script = script
	return f.err
}

func pairsOf(plan PlanView) []domain.Var {
	pairs := make([]domain.Var, len(plan.Vars))
	for i, resolved := range plan.Vars {
		pairs[i] = domain.Var{Key: resolved.Key, Value: resolved.Value}
	}
	return pairs
}

type fakeEnvironment []string

func (e fakeEnvironment) Environ() []string {
	return e
}

type fakeTemplates struct {
	content map[string]string
	err     error
	paths   []string
}

func (f *fakeTemplates) ReadTemplate(path string) ([]byte, bool, error) {
	f.paths = append(f.paths, path)
	if f.err != nil {
		return nil, false, f.err
	}
	content, ok := f.content[path]
	return []byte(content), ok, nil
}

type fakeProcesses struct {
	status ProcessStatus
	err    error
	spec   ProcessSpec
	calls  int
}

func (f *fakeProcesses) Run(spec ProcessSpec) (ProcessStatus, error) {
	f.calls++
	f.spec = spec
	return f.status, f.err
}
