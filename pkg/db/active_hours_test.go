package db

import (
	"testing"
	"time"
)

func TestParseAndInActiveHours(t *testing.T) {
	if !InActiveHours("", time.Now()) {
		t.Fatal("empty should always be active")
	}
	if _, err := ParseActiveHours("09:00-17:00"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateActiveHours("bad"); err == nil {
		t.Fatal("expected error")
	}
	// Fixed UTC noon
	noon := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	if !InActiveHours("09:00-17:00", noon) {
		t.Fatal("noon should be inside 09-17")
	}
	if InActiveHours("09:00-17:00", time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)) {
		t.Fatal("08:00 outside")
	}
	// overnight
	if !InActiveHours("22:00-06:00", time.Date(2026, 7, 16, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("23:00 should be in overnight window")
	}
	if !InActiveHours("22:00-06:00", time.Date(2026, 7, 16, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("03:00 should be in overnight window")
	}
	if InActiveHours("22:00-06:00", noon) {
		t.Fatal("noon outside overnight")
	}
	// multi (end is exclusive: 12:00 is not in 09:00-12:00)
	if !InActiveHours("09:00-12:00,13:00-17:00", time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)) {
		t.Fatal("11:00 should be in first window of multi")
	}
	if !InActiveHours("09:00-12:00,13:00-17:00", time.Date(2026, 7, 16, 14, 0, 0, 0, time.UTC)) {
		t.Fatal("14:00 should be in second window")
	}
	if InActiveHours("09:00-12:00,13:00-17:00", time.Date(2026, 7, 16, 12, 30, 0, 0, time.UTC)) {
		t.Fatal("12:30 lunch break")
	}
}

func TestReverseRule(t *testing.T) {
	r := &SyncRule{
		Name: "pipe", SourceProviderID: "s", SourceBucket: "in", SourcePrefix: "a/",
		TargetProviderID: "t", TargetBucket: "out", TargetPrefix: "b/",
		DeleteOnTarget: true, ExtraTargets: "x", Bidirectional: true,
	}
	rev := ReverseRule(r)
	if rev.SourceBucket != "out" || rev.TargetBucket != "in" {
		t.Fatalf("buckets: %+v", rev)
	}
	if rev.SourcePrefix != "b/" || rev.TargetPrefix != "a/" {
		t.Fatalf("prefixes: %+v", rev)
	}
	if rev.DeleteOnTarget || rev.ExtraTargets != "" || rev.Bidirectional {
		t.Fatalf("safety flags wrong: %+v", rev)
	}
	if rev.Name != "pipe (reverse)" {
		t.Fatalf("name=%q", rev.Name)
	}
}
