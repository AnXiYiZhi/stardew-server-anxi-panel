package web

import (
	"testing"
	"time"
)

func TestSameDiagnosticSaveIdentity(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"Farm", "Farm", true},
		{"Farm", "Farm_442923526", true},
		{"Farm_442923526", "Farm", true},
		{"Farm", "Farm_backup", false},
		{"", "Farm", false},
	}
	for _, tc := range cases {
		if got := sameDiagnosticSaveIdentity(tc.left, tc.right); got != tc.want {
			t.Fatalf("sameDiagnosticSaveIdentity(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestDurationBetween(t *testing.T) {
	start := "2026-07-11T10:00:00Z"
	end := "2026-07-11T10:02:30Z"
	got := durationBetween(start, end)
	if got == nil || *got != (150*time.Second).Milliseconds() {
		t.Fatalf("durationBetween = %v, want 150000", got)
	}
	if durationBetween(end, start) != nil || durationBetween("bad", end) != nil {
		t.Fatal("invalid or reversed timestamps must not produce a duration")
	}
	if durationBetween(start, "2026-07-11T14:02:30Z") != nil {
		t.Fatal("timestamps from different startup observations must not produce a duration")
	}
}
