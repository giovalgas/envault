package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type SaveSelection struct {
	store SelectionStore
	clock domain.Clock
}

func NewSaveSelection(store SelectionStore, clock domain.Clock) *SaveSelection {
	return &SaveSelection{store: store, clock: clock}
}

func (uc *SaveSelection) Execute(ctx context.Context, names []string) (domain.Selection, error) {
	selection, err := domain.NewSelection(names, uc.clock.Now())
	if err != nil {
		return domain.Selection{}, err
	}
	if err := uc.store.Save(ctx, selection); err != nil {
		return domain.Selection{}, err
	}
	return selection, nil
}
