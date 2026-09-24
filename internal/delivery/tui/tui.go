package tui

import (
	"context"

	"github.com/giovalgas/envault/internal/delivery/tui/app"
)

type (
	Store             = app.Store
	SelectionStore    = app.SelectionStore
	SavedSelection    = app.SavedSelection
	VaultStore        = app.VaultStore
	SelectionUseCases = app.SelectionUseCases
	Deps              = app.Deps
	Options           = app.Options
	Action            = app.Action
	ActionHandler     = app.ActionHandler
	ActionRequest     = app.ActionRequest
	ResultMsg         = app.ResultMsg
	InitVaultFunc     = app.InitVaultFunc
)

const (
	ActionNew     = app.ActionNew
	ActionEdit    = app.ActionEdit
	ActionImport  = app.ActionImport
	ActionCompose = app.ActionCompose
)

func Run(ctx context.Context, store Store, opts Options) error {
	return app.Run(ctx, store, opts)
}

func Actions(deps Deps) map[Action]ActionHandler {
	return app.Actions(deps)
}
