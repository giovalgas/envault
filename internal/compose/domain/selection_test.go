package domain

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestNewSelectionKeepsOrder(t *testing.T) {
	at := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	sel, err := NewSelection([]string{"b", "a"}, at)
	if err != nil {
		t.Fatalf("NewSelection: %v", err)
	}
	if !slices.Equal(sel.Envs, []string{"b", "a"}) || !sel.UpdatedAt.Equal(at) || !sel.Saved() {
		t.Fatalf("seleção = %+v", sel)
	}
}

func TestNewSelectionEmpty(t *testing.T) {
	sel, err := NewSelection(nil, time.Time{})
	if err != nil || len(sel.Envs) != 0 || sel.Envs == nil || sel.Saved() {
		t.Fatalf("seleção = %+v, err = %v", sel, err)
	}
}

func TestNewSelectionRejectsInvalidNames(t *testing.T) {
	for _, names := range [][]string{{"a", "a"}, {""}, {" a"}, {"a "}} {
		if _, err := NewSelection(names, time.Time{}); !errors.Is(err, ErrInvalidSelection) {
			t.Errorf("%q: err = %v, esperado ErrInvalidSelection", names, err)
		}
	}
}

func TestSelectionSplitSeparatesMissing(t *testing.T) {
	at := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	sel := Selection{Envs: []string{"c", "a", "x", "b"}, UpdatedAt: at}
	kept, missing := sel.Split([]string{"a", "b", "c"})
	if !slices.Equal(kept.Envs, []string{"c", "a", "b"}) || !kept.UpdatedAt.Equal(at) {
		t.Fatalf("mantidas = %+v", kept)
	}
	if !slices.Equal(missing, []string{"x"}) {
		t.Fatalf("missing = %v", missing)
	}
	kept, missing = Selection{}.Split(nil)
	if kept.Envs == nil || missing == nil || len(kept.Envs) != 0 || len(missing) != 0 {
		t.Fatalf("vazia: %+v %v", kept, missing)
	}
}

func TestClockNow(t *testing.T) {
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 500, time.FixedZone("x", 3600))
	if got := Clock(func() time.Time { return fixed }).Now(); !got.Equal(fixed.Truncate(time.Second)) || got.Location() != time.UTC {
		t.Fatalf("Now = %v", got)
	}
	var zero Clock
	if zero.Now().IsZero() {
		t.Fatal("relógio nulo deveria usar time.Now")
	}
}
