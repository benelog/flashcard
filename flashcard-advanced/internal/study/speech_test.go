package study

import "testing"

func TestMatchSpeech(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		heard    []string
		correct  bool
	}{
		{"그대로", "Have a good one!", []string{"have a good one"}, true},
		{"축약형을 풀어 말함", "It's my first day.", []string{"it is my first day"}, true},
		{"축약형으로 말함", "I am heading into work.", []string{"I'm heading into work"}, true},
		{"n't", "I didn't catch that.", []string{"I did not catch that"}, true},
		{"숫자를 숫자로 받아 적음", "Two fifty.", []string{"$2.50"}, true},
		{"두 자리 수", "Twenty five dollars", []string{"25 dollars"}, true},
		{"시각", "Nine thirty works.", []string{"9:30 works"}, true},
		{"동음이의", "I need to go too.", []string{"I need two go to"}, true},
		{"영국 철자", "My favorite color", []string{"my favourite colour"}, true},
		{"두 번째 후보가 맞음", "Knock 'em dead!", []string{"knock him dead", "knock em dead"}, true},
		{"정답이 여럿", "No kidding! / Seriously?", []string{"seriously"}, true},
		{"괄호는 말하지 않아도 됨", "pick up (the phone)", []string{"pick up"}, true},
		{"가벼운 말이 빠짐", "Can I get a medium latte, please?", []string{"can I get medium latte"}, true},
		{"them을 em으로 줄여 말함", "Knock them dead!", []string{"knock em dead"}, true},
		{"핵심어 하나가 틀림", "It's my first day.", []string{"it's my last day"}, false},
		{"핵심어 하나가 잘못 들림", "Where's the new gig?", []string{"where is the new kick"}, false},
		{"긴 문장에서 내용어 하나 빠짐", "I was wondering if you could send me the report before the meeting tomorrow.", []string{"I was wondering if you could send me the report before the meeting"}, true},
		{"짧은 표현에서 한 단어 빠짐", "No kidding!", []string{"no"}, false},
		{"다른 문장", "Where do I pay it?", []string{"what time is it"}, false},
		{"엉뚱한 말을 길게 덧붙임", "Thank you", []string{"thank you very much for everything you did today"}, false},
		{"들은 것이 없음", "Thank you", nil, false},
		{"한국어 띄어쓰기 차이", "출근하는 거야?", []string{"출근하는거야"}, true},
		{"한국어 다른 말", "출근하는 거야?", []string{"퇴근했어"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MatchSpeech(tt.expected, tt.heard)
			if m.Correct != tt.correct {
				t.Errorf("MatchSpeech(%q, %q) = %+v, want correct=%v", tt.expected, tt.heard, m, tt.correct)
			}
		})
	}
}

// 정답 단어마다 들렸는지 표시해 무엇을 빠뜨렸는지 보여 준다.
func TestMatchSpeechMarksWords(t *testing.T) {
	m := MatchSpeech("Where's the new gig?", []string{"where is the new kick"})
	want := []SpokenWord{{"Where's", true}, {"the", true}, {"new", true}, {"gig?", false}}
	if len(m.Words) != len(want) {
		t.Fatalf("words = %+v", m.Words)
	}
	for i := range want {
		if m.Words[i] != want[i] {
			t.Errorf("word %d = %+v, want %+v", i, m.Words[i], want[i])
		}
	}
	if m.Score != 80 || m.Heard != "where is the new kick" {
		t.Errorf("score=%d heard=%q, want 80 and the transcript", m.Score, m.Heard)
	}

	// 아무것도 못 들어도 정답 단어는 모두 (안 들린 것으로) 늘어놓는다.
	if m := MatchSpeech("Thank you", nil); len(m.Words) != 2 || m.Words[0].Heard || m.Score != 0 {
		t.Errorf("empty = %+v", m)
	}
}
