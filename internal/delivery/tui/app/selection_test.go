package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/giovalgas/envault/internal/delivery/tui/theme"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
	"github.com/giovalgas/envault/internal/shared/config"
)

const maxFooterBindings = 6

func (s stack) selectionPath() string {
	return filepath.Join(s.vaultDir, config.SelectionFileName)
}

func (s stack) savedNames(t *testing.T) []string {
	t.Helper()
	var doc struct {
		Envs []string `json:"envs"`
	}
	if err := json.Unmarshal([]byte(readFile(t, s.selectionPath())), &doc); err != nil {
		t.Fatalf("selection.json inválido: %v", err)
	}
	return doc.Envs
}

func (s stack) commandView(t *testing.T) ([]string, []string) {
	t.Helper()
	result, err := s.selection.GetSelection.Execute(context.Background())
	if err != nil {
		t.Fatalf("GetSelection: %v", err)
	}
	return result.Selection.Envs, result.Missing
}

func (s *session) waitSeen(text string) {
	s.t.Helper()
	if strings.Contains(s.seen.String(), text) {
		return
	}
	s.waitFor(text)
}

func rowWith(view, text string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, text) {
			return line
		}
	}
	return ""
}

func footerLine(view string) string {
	lines := strings.Split(view, "\n")
	return lines[len(lines)-1]
}

func assertFooter(t *testing.T, m Model, want ...key.Binding) {
	t.Helper()
	short := m.activeHelp().ShortHelp()
	if len(short) > maxFooterBindings {
		t.Fatalf("rodapé com %d atalhos, máximo %d", len(short), maxFooterBindings)
	}
	footer := footerLine(m.View())
	if n := strings.Count(footer, "•") + 1; n != len(short) || n > maxFooterBindings {
		t.Fatalf("rodapé renderizado com %d atalhos: %q", n, footer)
	}
	var keys []string
	for _, b := range short {
		keys = append(keys, b.Help().Key)
		if !strings.Contains(footer, b.Help().Key) || !strings.Contains(footer, b.Help().Desc) {
			t.Errorf("rodapé não mostra %q %q: %q", b.Help().Key, b.Help().Desc, footer)
		}
	}
	var wantKeys []string
	for _, b := range want {
		wantKeys = append(wantKeys, b.Help().Key)
	}
	if !slices.Equal(keys, wantKeys) {
		t.Fatalf("atalhos do rodapé = %v, esperado %v", keys, wantKeys)
	}
}

func assertFullHelp(t *testing.T, view string) {
	t.Helper()
	for _, group := range theme.DefaultKeyMap().AllGroups() {
		for _, b := range group {
			h := b.Help()
			if !strings.Contains(view, h.Key) || !strings.Contains(view, h.Desc) {
				t.Errorf("ajuda não mostra %q %q", h.Key, h.Desc)
			}
		}
	}
}

func TestSelectionSurvivesReopen(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	sess := s.start(t)
	sess.typeText("jj ")
	sess.waitFor("1 marcada")
	sess.typeText("kk ")
	sess.waitFor("2 marcadas")
	sess.typeText("q")
	sess.collect()

	if got := s.savedNames(t); !slices.Equal(got, []string{"stripe-test", "postgres-local"}) {
		t.Fatalf("selection.json = %v", got)
	}

	sess = s.start(t)
	sess.waitSeen("1. stripe-test  2. postgres-local")
	m, out := sess.finish()
	if !slices.Equal(m.list.Marked(), []string{"stripe-test", "postgres-local"}) {
		t.Fatalf("marcadas ao reabrir = %v", m.list.Marked())
	}
	view := m.View()
	if !strings.Contains(rowWith(view, "[1] stripe-test"), "stripe-test") || rowWith(view, "[2] postgres-local") == "" {
		t.Fatalf("lista sem a posição ao lado das marcadas:\n%s", view)
	}
	if !strings.Contains(rowWith(view, "redis"), viewmodel.MarkOff) {
		t.Fatalf("redis deveria aparecer desmarcada:\n%s", view)
	}
	if !strings.Contains(view, "seleção, a última vence: 1. stripe-test  2. postgres-local") {
		t.Fatalf("painel da seleção ausente:\n%s", view)
	}
	assertNoSecrets(t, "saída ao reabrir", out)
}

func TestSelectionUnmarkSaves(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	sess := s.start(t)
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("k ")
	sess.waitFor("1 marcada")
	sess.typeText("q")
	sess.collect()

	if got := s.savedNames(t); !slices.Equal(got, []string{"redis"}) {
		t.Fatalf("selection.json = %v", got)
	}
	if envs, missing := s.commandView(t); !slices.Equal(envs, []string{"redis"}) || len(missing) != 0 {
		t.Fatalf("comando veria %v, missing %v", envs, missing)
	}
}

func TestSelectionFileHasNoValues(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	sess := s.start(t)
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("q")
	sess.collect()

	data := readFile(t, s.selectionPath())
	assertNoSecrets(t, "selection.json", data)
	for _, key := range []string{"DATABASE_URL", "PGPASSWORD", "REDIS_URL"} {
		if strings.Contains(data, key) {
			t.Fatalf("selection.json contém a chave %s: %s", key, data)
		}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatalf("selection.json inválido: %v", err)
	}
	fields := make([]string, 0, len(raw))
	for field := range raw {
		fields = append(fields, field)
	}
	slices.Sort(fields)
	if !slices.Equal(fields, []string{"envs", "schema_version", "updated_at"}) {
		t.Fatalf("campos de selection.json = %v", fields)
	}
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX não se aplicam no windows")
	}
	info, err := os.Stat(s.selectionPath())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != config.FilePerm {
		t.Fatalf("permissão = %o, esperado %o", perm, config.FilePerm)
	}
}

func TestSelectionRenameSelectedEnv(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	sess := s.start(t)
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("kr")
	sess.waitFor("Renomear postgres-local")
	sess.press(tea.KeyCtrlU)
	sess.typeText("pg")
	sess.press(tea.KeyEnter)
	sess.waitFor("renomeada para pg")
	sess.waitSeen("1. pg  2. redis")
	sess.typeText("q")
	m, _ := sess.collect()

	if !slices.Equal(m.list.Marked(), []string{"pg", "redis"}) {
		t.Fatalf("marcadas = %v", m.list.Marked())
	}
	if got := s.savedNames(t); !slices.Equal(got, []string{"pg", "redis"}) {
		t.Fatalf("selection.json = %v", got)
	}
}

func TestSelectionDeleteSelectedEnv(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	sess := s.start(t)
	sess.typeText(" j ")
	sess.waitFor("2 marcadas")
	sess.typeText("kd")
	sess.waitFor("Apagar postgres-local")
	sess.typeText("postgres-local")
	sess.press(tea.KeyEnter)
	sess.waitFor("postgres-local apagada")
	sess.waitSeen("1. redis")
	sess.typeText("q")
	m, _ := sess.collect()

	if !slices.Equal(m.list.Marked(), []string{"redis"}) {
		t.Fatalf("marcadas = %v", m.list.Marked())
	}
	if got := s.savedNames(t); !slices.Equal(got, []string{"redis"}) {
		t.Fatalf("selection.json = %v", got)
	}
}

func TestSelectionComposeReorderSaves(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := openComposeAB(t, s)
	sess.typeText("J")
	sess.waitFor("prévia: b, a")
	sess.press(tea.KeyEsc)
	sess.waitSeen("1. b  2. a")
	sess.typeText("q")
	m, _ := sess.collect()

	if !slices.Equal(m.list.Marked(), []string{"b", "a"}) {
		t.Fatalf("marcadas = %v", m.list.Marked())
	}
	if got := s.savedNames(t); !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("selection.json = %v", got)
	}
	if envs, _ := s.commandView(t); !slices.Equal(envs, []string{"b", "a"}) {
		t.Fatalf("comando veria %v", envs)
	}

	sess = s.start(t)
	sess.waitSeen("1. b  2. a")
	sess.typeText("l")
	sess.waitFor("prévia: b, a")
	sess.finish()
}

func TestSelectionRestoreDropsMissingNames(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	writeFile(t, s.selectionPath(), `{"schema_version":1,"updated_at":"2026-09-23T12:00:00Z","envs":["redis","sumiu","postgres-local"]}`)
	sess := s.start(t)
	sess.waitSeen("removidas da seleção: sumiu")
	sess.typeText("q")
	m, _ := sess.collect()

	if !slices.Equal(m.list.Marked(), []string{"redis", "postgres-local"}) {
		t.Fatalf("marcadas = %v", m.list.Marked())
	}
	if got := s.savedNames(t); !slices.Equal(got, []string{"redis", "postgres-local"}) {
		t.Fatalf("selection.json = %v", got)
	}
}

func TestSelectionRestoreKeepsFileWhenUnchanged(t *testing.T) {
	s := newStack(t, seedEnvs()...)
	content := `{"schema_version":1,"updated_at":"2026-09-23T12:00:00Z","envs":["redis"]}`
	writeFile(t, s.selectionPath(), content)
	sess := s.start(t)
	sess.waitSeen("1. redis")
	sess.typeText("q")
	sess.collect()

	if got := readFile(t, s.selectionPath()); got != content {
		t.Fatalf("selection.json regravado sem mudança: %s", got)
	}
}

type fakeSelection struct {
	mu      sync.Mutex
	loadErr error
	saveErr error
	saved   [][]string
}

func (f *fakeSelection) Load(context.Context) (SavedSelection, error) {
	return SavedSelection{}, f.loadErr
}

func (f *fakeSelection) Save(_ context.Context, names []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, names)
	return f.saveErr
}

func TestSelectionLoadErrorIsShown(t *testing.T) {
	store := &fakeSelection{loadErr: errors.New("selection.json ilegível")}
	sess := startSession(t, newTestVault(t), Options{Selection: store})
	sess.waitSeen("ler seleção: selection.json ilegível")
	m, _ := sess.finish()
	if !m.statusErr || len(m.list.Marked()) != 0 {
		t.Fatalf("status = %q, marcadas = %v", m.status, m.list.Marked())
	}
	if len(store.saved) != 0 {
		t.Fatalf("seleção ilegível não deveria ser sobrescrita ao abrir: %v", store.saved)
	}
}

func TestSelectionSaveErrorIsShown(t *testing.T) {
	store := &fakeSelection{saveErr: errors.New("disco cheio")}
	sess := startSession(t, newTestVault(t), Options{Selection: store})
	sess.typeText(" ")
	sess.waitSeen("gravar seleção: disco cheio")
	m, _ := sess.finish()
	if !m.statusErr {
		t.Fatal("status deveria indicar erro")
	}
}

func TestSelectionSaverSkipsStaleWrites(t *testing.T) {
	store := &fakeSelection{}
	saver := newSelectionSaver(store)
	saver.pending.Add(2)
	if err := saver.save(context.Background(), 2, []string{"b"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := saver.save(context.Background(), 1, []string{"a"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	saver.wait()
	if len(store.saved) != 1 || !slices.Equal(store.saved[0], []string{"b"}) {
		t.Fatalf("gravações = %v", store.saved)
	}
	if newSelectionSaver(nil) != nil {
		t.Fatal("sem store não deveria haver saver")
	}
}

func TestFooterList(t *testing.T) {
	sess := startSession(t, newTestVault(t), Options{})
	m, _ := sess.finish()
	k := theme.DefaultKeyMap()
	assertFooter(t, m, k.Mark, k.Compose, k.New, k.Filter, k.Help, k.Quit)
	footer := footerLine(m.View())
	for _, want := range []string{"space marcar", "l carregar", "n nova env", "/ filtrar", "? ajuda", "q/ctrl+c sair"} {
		if !strings.Contains(footer, want) {
			t.Errorf("rodapé da lista sem %q: %q", want, footer)
		}
	}
}

func TestFooterDetail(t *testing.T) {
	sess := openDetail(t, Options{})
	m, _ := sess.finish()
	if m.screen != screenDetail {
		t.Fatalf("tela = %v", m.screen)
	}
	k := theme.DefaultKeyMap()
	assertFooter(t, m, k.Up, k.Down, k.Reveal, k.Copy, k.Help, k.Back)
}

func TestFooterCompose(t *testing.T) {
	s := newStack(t, composeEnvs()...)
	sess := openComposeAB(t, s)
	m, _ := sess.finish()
	if m.screen != screenCompose {
		t.Fatalf("tela = %v", m.screen)
	}
	k := theme.DefaultKeyMap()
	assertFooter(t, m, k.MoveUp, k.MoveDown, k.NextPane, k.Write, k.Help, k.Back)
}

func TestHelpListsAllShortcutsOnEveryScreen(t *testing.T) {
	t.Run("lista", func(t *testing.T) {
		sess := startSession(t, newTestVault(t), Options{})
		sess.typeText("?")
		sess.waitFor("Atalhos")
		m, _ := sess.finish()
		assertFullHelp(t, m.View())
	})
	t.Run("detalhe", func(t *testing.T) {
		sess := openDetail(t, Options{})
		sess.typeText("?")
		sess.waitFor("Atalhos")
		m, _ := sess.finish()
		if m.screen != screenDetail {
			t.Fatalf("tela = %v", m.screen)
		}
		assertFullHelp(t, m.View())
	})
	t.Run("montagem", func(t *testing.T) {
		s := newStack(t, composeEnvs()...)
		sess := openComposeAB(t, s)
		sess.typeText("?")
		sess.waitFor("Atalhos")
		m, _ := sess.finish()
		if m.screen != screenCompose {
			t.Fatalf("tela = %v", m.screen)
		}
		assertFullHelp(t, m.View())
	})
}

func TestSelectionHiddenOutsideList(t *testing.T) {
	sess := openDetail(t, Options{})
	m, _ := sess.finish()
	if strings.Contains(m.View(), "seleção") {
		t.Fatalf("painel da seleção fora da lista:\n%s", m.View())
	}
}
