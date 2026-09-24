package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/giovalgas/envault/internal/compose/domain"
)

type fakeCatalog struct {
	names []string
	err   error
}

func (c *fakeCatalog) EnvNames(context.Context) ([]string, error) {
	return c.names, c.err
}

type fakeSelections struct {
	stored  domain.Selection
	loadErr error
	saveErr error
	loads   int
	saves   int
}

func (s *fakeSelections) Load(context.Context) (domain.Selection, error) {
	s.loads++
	return s.stored, s.loadErr
}

func (s *fakeSelections) Save(_ context.Context, selection domain.Selection) error {
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.stored = selection
	return nil
}

var selectionTime = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func TestGetSelectionSeparatesMissing(t *testing.T) {
	store := &fakeSelections{stored: domain.Selection{Envs: []string{"b", "gone", "a"}, UpdatedAt: selectionTime}}
	uc := NewGetSelection(&fakeCatalog{names: []string{"a", "b", "c"}}, store)
	result, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !slices.Equal(result.Selection.Envs, []string{"b", "a"}) || !slices.Equal(result.Missing, []string{"gone"}) {
		t.Fatalf("resultado = %+v", result)
	}
	if !result.Selection.UpdatedAt.Equal(selectionTime) {
		t.Fatalf("updated_at = %v", result.Selection.UpdatedAt)
	}
}

func TestGetSelectionWithoutSavedSelection(t *testing.T) {
	uc := NewGetSelection(&fakeCatalog{names: []string{"a"}}, &fakeSelections{})
	result, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Selection.Envs) != 0 || len(result.Missing) != 0 || result.Selection.Saved() {
		t.Fatalf("resultado = %+v", result)
	}
}

func TestGetSelectionChecksVaultBeforeReading(t *testing.T) {
	store := &fakeSelections{}
	uc := NewGetSelection(&fakeCatalog{err: errBoom}, store)
	if _, err := uc.Execute(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if store.loads != 0 {
		t.Fatalf("seleção lida %d vezes com o cofre indisponível", store.loads)
	}
}

func TestGetSelectionLoadError(t *testing.T) {
	uc := NewGetSelection(&fakeCatalog{}, &fakeSelections{loadErr: errBoom})
	if _, err := uc.Execute(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveSelectionStampsTime(t *testing.T) {
	store := &fakeSelections{}
	uc := NewSaveSelection(store, func() time.Time { return selectionTime })
	saved, err := uc.Execute(context.Background(), []string{"b", "a"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !slices.Equal(store.stored.Envs, []string{"b", "a"}) || !store.stored.UpdatedAt.Equal(selectionTime) || !slices.Equal(saved.Envs, store.stored.Envs) {
		t.Fatalf("gravada = %+v, devolvida = %+v", store.stored, saved)
	}
}

func TestSaveSelectionRejectsDuplicates(t *testing.T) {
	store := &fakeSelections{}
	uc := NewSaveSelection(store, nil)
	if _, err := uc.Execute(context.Background(), []string{"a", "a"}); !errors.Is(err, domain.ErrInvalidSelection) {
		t.Fatalf("err = %v", err)
	}
	if store.saves != 0 {
		t.Fatal("seleção inválida não deveria ser gravada")
	}
}

func TestSaveSelectionStoreError(t *testing.T) {
	uc := NewSaveSelection(&fakeSelections{saveErr: errBoom}, nil)
	if _, err := uc.Execute(context.Background(), nil); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}
