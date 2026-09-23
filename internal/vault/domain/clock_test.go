package domain

import (
	"testing"
	"time"
)

func TestClockNow(t *testing.T) {
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 999, time.FixedZone("BRT", -3*3600))
	got := Clock(func() time.Time { return fixed }).Now()
	if !got.Equal(fixed.Truncate(time.Second)) || got.Location() != time.UTC {
		t.Fatalf("Now = %v", got)
	}
	var zero Clock
	if since := time.Since(zero.Now()); since < 0 || since > time.Minute {
		t.Fatalf("nil clock Now = %v", zero.Now())
	}
}
