package app

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/tui/screen/compose"
	"github.com/giovalgas/envault/internal/delivery/tui/screen/confirm"
	"github.com/giovalgas/envault/internal/delivery/tui/viewmodel"
)

func (m Model) openCompose(msg composeOpenMsg) (tea.Model, tea.Cmd) {
	msg.deps.Clipboard = m.clipboard
	next, cmd := compose.New(m.ctx, msg.names, msg.deps).PlanCmd()
	m.compose = next
	m.screen = screenCompose
	m = m.setStatus("", nil)
	return m, cmd
}

func (m Model) onComposePlan(msg compose.PlanMsg) Model {
	if m.screen != screenCompose {
		return m
	}
	next, current := m.compose.OnPlan(msg)
	m.compose = next
	if current && msg.Err != nil {
		m.logger.Printf("montar prévia: %v", msg.Err)
		return m.setStatus("", fmt.Errorf("montar prévia: %w", msg.Err))
	}
	return m
}

func (m Model) closeCompose() Model {
	m.screen = screenList
	m.compose = compose.Model{}
	return m
}

func (m Model) updateCompose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	next, cmd, effect := m.compose.Update(msg)
	m.compose = next
	switch effect.Intent {
	case compose.IntentClose:
		return m.closeCompose(), nil
	case compose.IntentHelp:
		m.showHelp = true
	case compose.IntentReordered:
		before := m.list.Marked()
		m.list = m.list.WithMarked(m.compose.Order())
		var save tea.Cmd
		m, save = m.persistSelection(before)
		return m, tea.Batch(cmd, save)
	case compose.IntentStatus:
		m = m.applyComposeStatus(effect)
	}
	return m, cmd
}

func (m Model) applyComposeStatus(effect compose.Effect) Model {
	switch {
	case effect.Err != nil:
		return m.setStatus("", effect.Err)
	case effect.Warning != "":
		return m.setWarning(effect.Warning)
	}
	return m.setStatus(effect.Status, nil)
}

func (m Model) onComposeExported(msg compose.ExportedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.logger.Printf("exportar montagem: %v", msg.Err)
		return m.setStatus("", fmt.Errorf("exportar no terminal: %w", msg.Err)), nil
	}
	m.exported = viewmodel.ExportedText(msg.Result)
	return m, m.quitCmd()
}

func (m Model) onComposeCopied(msg compose.CopiedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.logger.Printf("copiar montagem: %v", msg.Err)
		return m.setStatus("", fmt.Errorf("copiar para o clipboard: %w", msg.Err)), nil
	}
	m = m.closeCompose()
	return m.onResult(ResultMsg{
		Status:  viewmodel.CopiedText(msg.Result),
		Warning: viewmodel.CopiedWarning(msg.Result),
	})
}

func (m Model) onComposeWritten(msg compose.WrittenMsg) (tea.Model, tea.Cmd) {
	switch {
	case errors.Is(msg.Err, composeusecase.ErrTargetExists):
		return m.openTargetExists(msg.Target), nil
	case errors.Is(msg.Err, composeusecase.ErrTargetIsDirectory):
		return m.setStatus("", fmt.Errorf("destino %s é um diretório", msg.Target)), nil
	case msg.Err != nil:
		m.logger.Printf("gravar montagem: %v", msg.Err)
		return m.setStatus("", fmt.Errorf("gravar %s: %w", msg.Target, msg.Err)), nil
	}
	m = m.closeCompose()
	return m.onResult(ResultMsg{
		Status:  viewmodel.WrittenText(msg.Target, msg.Result),
		Warning: viewmodel.WrittenWarning(msg.Target, msg.Result),
	})
}

func (m Model) openTargetExists(target string) Model {
	current := m.compose
	write := func(existing composeusecase.ExistingTarget) func(string) (tea.Cmd, error) {
		return func(string) (tea.Cmd, error) { return current.WriteCmd(existing) }
	}
	modal := confirm.New("Destino já existe",
		fmt.Sprintf("%s já existe. Sobrescrever troca o arquivo inteiro; mesclar mantém as chaves locais e atualiza as das envs.", target)).
		WithChoices(
			confirm.Choice{Label: "sobrescrever", Run: write(composeusecase.ReplaceExisting)},
			confirm.Choice{Label: "mesclar", Run: write(composeusecase.MergeExisting)},
			confirm.Cancel(),
		)
	m.modal = &modal
	return m.setStatus("", nil)
}
