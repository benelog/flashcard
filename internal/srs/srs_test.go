package srs

import (
	"testing"
	"time"
)

// tag::correct-progression[]
var now = time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

func TestCorrectProgression(t *testing.T) {
	s := NewState()
	// ease: 2.5→2.6→2.7→2.8; rep3: round(6×2.7)=16, rep4: round(16×2.8)=45
	wantIntervals := []float64{1, 6, 16, 45}
	for i, want := range wantIntervals {
		var due time.Time
		s, due = Grade(s, true, now)
		if s.IntervalDays != want {
			t.Fatalf("rep %d: interval = %v, want %v", i+1, s.IntervalDays, want)
		}
		wantDue := now.Add(time.Duration(want * float64(24*time.Hour)))
		if !due.Equal(wantDue) {
			t.Fatalf("rep %d: due = %v, want %v", i+1, due, wantDue)
		}
	}
	// end::correct-progression[]
	if s.Repetitions != 4 {
		t.Fatalf("repetitions = %d, want 4", s.Repetitions)
	}
}

func TestLapseResets(t *testing.T) {
	s := NewState()
	s, _ = Grade(s, true, now)
	s, _ = Grade(s, true, now)
	s, _ = Grade(s, false, now)
	if s.Repetitions != 0 {
		t.Fatalf("repetitions = %d, want 0", s.Repetitions)
	}
	if s.IntervalDays != 1 {
		t.Fatalf("interval = %v, want 1", s.IntervalDays)
	}
	// 2.5 +0.1 +0.1 -0.32 = 2.38
	if diff := s.EaseFactor - 2.38; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("ease = %v, want 2.38", s.EaseFactor)
	}
	// Relearn: intervals restart at 1, 6.
	s, _ = Grade(s, true, now)
	if s.IntervalDays != 1 {
		t.Fatalf("relearn interval = %v, want 1", s.IntervalDays)
	}
}

// tag::ease-floor[]
func TestEaseFloor(t *testing.T) {
	s := NewState()
	for range 10 {
		s, _ = Grade(s, false, now)
	}
	if s.EaseFactor != MinEase {
		t.Fatalf("ease = %v, want floor %v", s.EaseFactor, MinEase)
	}
}

// end::ease-floor[]
