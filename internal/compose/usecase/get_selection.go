package usecase

import (
	"context"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type GetSelection struct {
	catalog EnvCatalog
	store   SelectionStore
}

type GetSelectionResult struct {
	Selection domain.Selection
	Missing   []string
}

func NewGetSelection(catalog EnvCatalog, store SelectionStore) *GetSelection {
	return &GetSelection{catalog: catalog, store: store}
}

func (uc *GetSelection) Execute(ctx context.Context) (GetSelectionResult, error) {
	existing, err := uc.catalog.EnvNames(ctx)
	if err != nil {
		return GetSelectionResult{}, err
	}
	stored, err := uc.store.Load(ctx)
	if err != nil {
		return GetSelectionResult{}, err
	}
	kept, missing := stored.Split(existing)
	return GetSelectionResult{Selection: kept, Missing: missing}, nil
}
