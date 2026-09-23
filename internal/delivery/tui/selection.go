package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

type SelectionStore interface {
	Load(ctx context.Context) (SavedSelection, error)
	Save(ctx context.Context, names []string) error
}

type SavedSelection struct {
	Envs    []string
	Missing []string
}

type selectionSaver struct {
	store   SelectionStore
	mu      sync.Mutex
	written int
	pending sync.WaitGroup
}

func newSelectionSaver(store SelectionStore) *selectionSaver {
	if store == nil {
		return nil
	}
	return &selectionSaver{store: store}
}

func (s *selectionSaver) load(ctx context.Context) (SavedSelection, error) {
	return s.store.Load(ctx)
}

func (s *selectionSaver) save(ctx context.Context, seq int, names []string) error {
	defer s.pending.Done()
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq <= s.written {
		return nil
	}
	if err := s.store.Save(ctx, names); err != nil {
		return err
	}
	s.written = seq
	return nil
}

func (s *selectionSaver) wait() {
	if s != nil {
		s.pending.Wait()
	}
}

type restoredSelection struct {
	saved SavedSelection
	err   error
}

type selectionSavedMsg struct {
	err error
}

type envRenamedMsg struct {
	oldName string
	newName string
	result  ResultMsg
}

func (m Model) firstLoadCmd(status string) tea.Cmd {
	if m.selection == nil {
		return m.loadCmd("", status)
	}
	ctx, store, saver := m.ctx, m.store, m.selection
	return func() tea.Msg {
		envs, err := store.List(ctx)
		msg := envsLoadedMsg{envs: envs, err: err, status: status}
		if err == nil {
			saved, loadErr := saver.load(ctx)
			msg.restore = &restoredSelection{saved: saved, err: loadErr}
		}
		return msg
	}
}

func (m Model) restoreSelection(msg envsLoadedMsg, before []string) (Model, []string) {
	restore := msg.restore
	if restore == nil {
		return m, before
	}
	if restore.err != nil {
		m.logger.Printf("ler seleção: %v", restore.err)
		return m.setStatus("", fmt.Errorf("ler seleção: %w", restore.err)), m.list.marked
	}
	m.list = m.list.withMarked(restore.saved.Envs)
	if missing := restore.saved.Missing; len(missing) > 0 {
		m = m.setWarning(joinStatus(msg.status, "fora do cofre, removidas da seleção: "+strings.Join(missing, ", ")))
	}
	return m, append(slices.Clone(restore.saved.Envs), restore.saved.Missing...)
}

func (m Model) persistSelection(before []string) (Model, tea.Cmd) {
	if m.selection == nil || slices.Equal(before, m.list.marked) {
		return m, nil
	}
	m.saveSeq++
	ctx, saver, seq, names := m.ctx, m.selection, m.saveSeq, slices.Clone(m.list.marked)
	saver.pending.Add(1)
	return m, func() tea.Msg {
		return selectionSavedMsg{err: saver.save(ctx, seq, names)}
	}
}

func (m Model) onSelectionSaved(msg selectionSavedMsg) Model {
	if msg.err == nil {
		return m
	}
	m.logger.Printf("gravar seleção: %v", msg.err)
	return m.setStatus("", fmt.Errorf("gravar seleção: %w", msg.err))
}

func (m Model) onEnvRenamed(msg envRenamedMsg) (tea.Model, tea.Cmd) {
	before := m.list.marked
	m.list = m.list.renameMarked(msg.oldName, msg.newName)
	m, save := m.persistSelection(before)
	return m, tea.Batch(save, m.reloadCmd(msg.result))
}

func (m Model) quitCmd() tea.Cmd {
	saver := m.selection
	if saver == nil {
		return tea.Quit
	}
	return tea.Sequence(func() tea.Msg {
		saver.wait()
		return nil
	}, tea.Quit)
}

func (m Model) showsSelection() bool {
	return m.screen == screenList && !m.showHelp
}

func (l listModel) selectionView(st styles, width int) string {
	if len(l.marked) == 0 {
		return st.subtle.Render(truncate("seleção: nenhuma env marcada", width))
	}
	items := make([]string, len(l.marked))
	for i, name := range l.marked {
		items[i] = fmt.Sprintf("%d. %s", i+1, name)
	}
	label := "seleção, a última vence: "
	return st.label.Render(truncate(label+strings.Join(items, "  "), width))
}
