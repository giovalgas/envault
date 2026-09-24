package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	"github.com/giovalgas/envault/internal/shared/config"
	vault "github.com/giovalgas/envault/internal/vault/domain"
	"github.com/giovalgas/envault/internal/vault/infra/encryptedfile"
	"github.com/giovalgas/envault/internal/vault/infra/keystore"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

const (
	waitTimeout = 5 * time.Second
	termWidth   = 120
	termHeight  = 32

	secretDatabaseURL = "valor-database"
	secretPGPassword  = "valor-password"
	secretStripeKey   = "valor-stripe"
	secretRedisURL    = "valor-redis"
)

var allSecrets = []string{secretDatabaseURL, secretPGPassword, secretStripeKey, secretRedisURL}

func seedEnvs() []vault.Env {
	return []vault.Env{
		{
			Name:        "postgres-local",
			Description: "Postgres local via docker-compose",
			Tags:        []string{"db", "local"},
			Vars: []vault.Var{
				{Key: "DATABASE_URL", Value: secretDatabaseURL},
				{Key: "PGPASSWORD", Value: secretPGPassword},
			},
		},
		{
			Name:        "redis",
			Description: "Cache",
			Tags:        []string{"cache"},
			Vars:        []vault.Var{{Key: "REDIS_URL", Value: secretRedisURL}},
		},
		{
			Name:        "stripe-test",
			Description: "Stripe modo teste",
			Tags:        []string{"payments"},
			Vars:        []vault.Var{{Key: "STRIPE_SECRET_KEY", Value: secretStripeKey}},
		},
	}
}

func envViews(envs []vault.Env) []vaultusecase.EnvView {
	views := make([]vaultusecase.EnvView, len(envs))
	for i, env := range envs {
		vars := make([]vaultusecase.VarView, len(env.Vars))
		for j, v := range env.Vars {
			vars[j] = vaultusecase.VarView(v)
		}
		views[i] = vaultusecase.EnvView{
			Name:        env.Name,
			Description: env.Description,
			Tags:        env.Tags,
			Vars:        vars,
			CreatedAt:   env.CreatedAt,
			UpdatedAt:   env.UpdatedAt,
		}
	}
	return views
}

func newTestVault(t *testing.T) VaultStore {
	t.Helper()
	key, err := vault.NewKey()
	if err != nil {
		t.Fatalf("gerar chave: %v", err)
	}
	keys, err := keystore.NewStatic(key)
	if err != nil {
		t.Fatalf("criar keystore: %v", err)
	}
	repo := encryptedfile.New(config.PathsIn(t.TempDir()), keys)
	ctx := context.Background()
	if _, err := repo.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, env := range seedEnvs() {
		if err := repo.Update(ctx, func(s *vault.Snapshot) error {
			_, err := s.Create(env, time.Now())
			return err
		}); err != nil {
			t.Fatalf("criar %s: %v", env.Name, err)
		}
	}
	return VaultStore{
		ListEnvs:  vaultusecase.NewListEnvs(repo),
		ShowEnv:   vaultusecase.NewShowEnv(repo),
		CopyEnv:   vaultusecase.NewCopyEnv(repo, nil),
		RenameEnv: vaultusecase.NewRenameEnv(repo, nil),
		DeleteEnv: vaultusecase.NewDeleteEnv(repo),
	}
}

type session struct {
	t    *testing.T
	tm   *teatest.TestModel
	seen bytes.Buffer
}

func startSession(t *testing.T, s Store, opts Options) *session {
	t.Helper()
	if opts.Clipboard == nil {
		opts.Clipboard = func(string) error { return errors.New("clipboard indisponível no teste") }
	}
	tm := teatest.NewTestModel(t, New(context.Background(), s, opts), teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor("postgres-local")
	return sess
}

func (s *session) waitFor(text string) {
	s.t.Helper()
	var last []byte
	teatest.WaitFor(s.t, s.tm.Output(), func(b []byte) bool {
		last = b
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(waitTimeout), teatest.WithCheckInterval(10*time.Millisecond))
	s.seen.Write(last)
}

func (s *session) typeText(text string) {
	s.tm.Type(text)
}

func (s *session) press(k tea.KeyType) {
	s.tm.Send(tea.KeyMsg{Type: k})
}

func (s *session) finish() (Model, string) {
	s.t.Helper()
	if err := s.tm.Quit(); err != nil {
		s.t.Fatalf("encerrar programa: %v", err)
	}
	return s.collect()
}

func (s *session) collect() (Model, string) {
	s.t.Helper()
	final := s.tm.FinalModel(s.t, teatest.WithFinalTimeout(waitTimeout))
	for deadline := time.Now().Add(waitTimeout); final == nil && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
		final = s.tm.FinalModel(s.t)
	}
	rest, err := io.ReadAll(s.tm.FinalOutput(s.t, teatest.WithFinalTimeout(waitTimeout)))
	if err != nil {
		s.t.Fatalf("ler saída: %v", err)
	}
	s.seen.Write(rest)
	m, ok := final.(Model)
	if !ok {
		s.t.Fatalf("modelo final inesperado: %T", final)
	}
	return m, s.seen.String()
}

func assertNoSecrets(t *testing.T, where, text string, secrets ...string) {
	t.Helper()
	if len(secrets) == 0 {
		secrets = allSecrets
	}
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("%s contém o valor %q", where, secret)
		}
	}
}

func TestAppListRendersWithoutValues(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText("q")
	m, out := sess.collect()

	assertNoSecrets(t, "saída renderizada", out)
	view := m.View()
	assertNoSecrets(t, "view final", view)
	for _, want := range []string{"postgres-local", "redis", "stripe-test", "DATABASE_URL", "PGPASSWORD", viewmodel.MaskedValue, "Postgres local via docker-compose", "db, local", "2 chaves"} {
		if !strings.Contains(view, want) {
			t.Errorf("view não contém %q:\n%s", want, view)
		}
	}
}

func TestAppHelpShowsAllShortcuts(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	sess.typeText("?")
	sess.waitFor("Atalhos")
	m, _ := sess.finish()

	view := m.View()
	keys := theme.DefaultKeyMap()
	for _, group := range keys.AllGroups() {
		for _, b := range group {
			h := b.Help()
			if !strings.Contains(view, h.Key) || !strings.Contains(view, h.Desc) {
				t.Errorf("ajuda não mostra %q %q", h.Key, h.Desc)
			}
		}
	}
	assertNoSecrets(t, "ajuda", view)
}

func TestAppQuitBehavior(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "q", key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}},
		{name: "ctrl+c", key: tea.KeyMsg{Type: tea.KeyCtrlC}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := startSession(t, newTestVault(t), Options{})
			sess.tm.Send(tt.key)
			m, _ := sess.collect()
			if m.screen != screenList {
				t.Fatalf("tela final = %v, esperado lista", m.screen)
			}
		})
	}
}

func TestAppComposeHookReceivesMarkedInOrder(t *testing.T) {
	var (
		mu  sync.Mutex
		got ActionRequest
	)
	opts := Options{Actions: map[Action]ActionHandler{
		ActionCompose: func(_ context.Context, req ActionRequest) tea.Cmd {
			mu.Lock()
			got = req
			mu.Unlock()
			return func() tea.Msg { return ResultMsg{Status: "montagem concluída"} }
		},
	}}
	sess := startSession(t, newTestVault(t), opts)
	sess.typeText("jj ")
	sess.waitFor("SELECTED_ENVS=stripe-test")
	sess.typeText("kk ")
	sess.waitFor("SELECTED_ENVS=stripe-test,postgres-local")
	sess.typeText("l")
	sess.waitFor("montagem concluída")
	sess.finish()

	mu.Lock()
	defer mu.Unlock()
	var names []string
	for _, env := range got.Marked {
		names = append(names, env.Name)
	}
	if strings.Join(names, ",") != "stripe-test,postgres-local" {
		t.Fatalf("marcadas = %v, esperado ordem de marcação", names)
	}
	if got.Focused == nil || got.Focused.Name != "postgres-local" {
		t.Fatalf("focada = %+v", got.Focused)
	}
}

func TestAppActionWithoutHandlerShowsStatus(t *testing.T) {
	for _, k := range []string{"n", "e", "i", "l"} {
		t.Run(k, func(t *testing.T) {
			sess := startSession(t, newTestVault(t), Options{})
			sess.typeText(k)
			m, _ := sess.finish()
			if !m.statusErr || m.status == "" {
				t.Fatalf("status = %q, esperado erro de ação indisponível", m.status)
			}
		})
	}
}

func TestAppActionResultErrorShowsStatus(t *testing.T) {
	opts := Options{Actions: map[Action]ActionHandler{
		ActionNew: func(context.Context, ActionRequest) tea.Cmd {
			return func() tea.Msg { return ResultMsg{Err: errors.New("editor cancelado")} }
		},
	}}
	sess := startSession(t, newTestVault(t), opts)
	sess.typeText("n")
	sess.waitFor("editor cancelado")
	m, _ := sess.finish()
	if !m.statusErr {
		t.Fatal("status deveria indicar erro")
	}
}

type failingStore struct {
	Store
}

func (failingStore) List(context.Context) ([]vaultusecase.EnvView, error) {
	return nil, vault.ErrDecrypt
}

func TestAppLoadErrorIsShown(t *testing.T) {
	tm := teatest.NewTestModel(t, New(context.Background(), failingStore{}, Options{}), teatest.WithInitialTermSize(termWidth, termHeight))
	sess := &session{t: t, tm: tm}
	sess.waitFor("não foi possível ler o cofre")
	m, _ := sess.finish()
	if !m.statusErr {
		t.Fatal("status deveria indicar erro")
	}
}

type signalingStore struct {
	Store
	listed chan struct{}
	once   sync.Once
}

func (s *signalingStore) List(ctx context.Context) ([]vaultusecase.EnvView, error) {
	envs, err := s.Store.List(ctx)
	s.once.Do(func() { close(s.listed) })
	return envs, err
}

func TestRunWritesDebugLogWithoutValues(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "debug.log")
	s := &signalingStore{Store: newTestVault(t), listed: make(chan struct{})}
	input, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), s, Options{Debug: true, LogPath: logPath, Input: input, Output: io.Discard})
	}()

	select {
	case <-s.listed:
	case <-time.After(waitTimeout):
		t.Fatal("TUI não carregou as envs")
	}
	deadline := time.Now().Add(waitTimeout)
	for {
		data, _ := os.ReadFile(filepath.Clean(logPath))
		if bytes.Contains(data, []byte("envs carregadas: 3")) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("log não registrou a carga: %q", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatalf("enviar tecla: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run não terminou")
	}
	_ = writer.Close()

	data, err := os.ReadFile(filepath.Clean(logPath))
	if err != nil {
		t.Fatalf("ler log: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(debugLogPrefix+" ")) {
		t.Fatalf("log sem prefixo: %q", data)
	}
	assertNoSecrets(t, "log de depuração", string(data))
}

func TestActionString(t *testing.T) {
	tests := map[Action]string{
		ActionNew:     "new",
		ActionEdit:    "edit",
		ActionImport:  "import",
		ActionCompose: "compose",
		Action(99):    "action(99)",
	}
	for action, want := range tests {
		if got := action.String(); got != want {
			t.Errorf("%d.String() = %q, esperado %q", int(action), got, want)
		}
	}
}
