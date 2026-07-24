// tag::head[]

// Package srs implements a binary-grade variant of SM-2. The UI only offers
// 맞음/틀림, mapped to SM-2 quality 5 and 2 respectively.
package srs

import (
	"math"
	"time"
)

const (
	MinEase     = 1.3
	InitialEase = 2.5
	// SM-2 ease deltas for q=5 and q=2.
	easeGainCorrect   = 0.1
	easeLossIncorrect = 0.32
)

// end::head[]

// tag::state[]
type State struct {
	EaseFactor   float64
	IntervalDays float64
	Repetitions  int
}

func NewState() State {
	return State{EaseFactor: InitialEase}
}

// end::state[]

// tag::grade[]
// Grade returns the next SRS state and due time after a first-pass answer.
// Retry-round answers must not be graded.
func Grade(s State, correct bool, now time.Time) (State, time.Time) {
	if correct {
		s.Repetitions++
		switch s.Repetitions {
		case 1:
			s.IntervalDays = 1
		case 2:
			s.IntervalDays = 6
		default:
			s.IntervalDays = math.Round(s.IntervalDays * s.EaseFactor)
		}
		s.EaseFactor += easeGainCorrect
	} else {
		s.Repetitions = 0
		s.IntervalDays = 1
		s.EaseFactor = math.Max(MinEase, s.EaseFactor-easeLossIncorrect)
	}
	due := now.Add(time.Duration(s.IntervalDays * float64(24*time.Hour)))
	return s, due
}

// end::grade[]
