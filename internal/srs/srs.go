// tag::head[]

// Package srs는 SM-2의 이진 채점 변형을 구현한다. UI는 맞음/틀림만 내놓고,
// 각각 SM-2 품질 5와 2에 대응한다.
package srs

import (
	"math"
	"time"
)

const (
	MinEase     = 1.3
	InitialEase = 2.5
	// q=5와 q=2에 해당하는 SM-2의 ease 증감.
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
// Grade는 첫 회차 답변 뒤의 다음 SRS 상태와 복습 예정 시각을 돌려준다.
// 재도전 회차의 답변은 채점하면 안 된다.
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
