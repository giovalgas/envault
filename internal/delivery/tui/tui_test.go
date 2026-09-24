package tui

import (
	"testing"

	composeusecase "github.com/giovalgas/envault/internal/compose/usecase"
	"github.com/giovalgas/envault/internal/delivery/tui/app"
	vaultusecase "github.com/giovalgas/envault/internal/vault/usecase"
)

var (
	_ Store          = VaultStore{}
	_ SelectionStore = SelectionUseCases{}
	_ app.Options    = Options{}
	_ app.Deps       = Deps{}
)

func TestFacadeActionsRegistersOnlyAvailableUseCases(t *testing.T) {
	if got := Actions(Deps{}); len(got) != 0 {
		t.Fatalf("Actions vazio = %v", got)
	}
	got := Actions(Deps{
		BeginCreateEnv: &vaultusecase.BeginCreateEnv{},
		LoadTemplate:   &composeusecase.LoadTemplate{},
		PlanLoad:       &composeusecase.PlanLoad{},
		LoadEnvFile:    &composeusecase.LoadEnvFile{},
	})
	for _, action := range []Action{ActionNew, ActionCompose} {
		if got[action] == nil {
			t.Errorf("ação %s não registrada", action)
		}
	}
	for _, action := range []Action{ActionEdit, ActionImport} {
		if got[action] != nil {
			t.Errorf("ação %s registrada sem use case", action)
		}
	}
}
