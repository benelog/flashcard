package web

import (
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/model"
)

// 저장소가 UTC로 읽어 온 공유 시각을 방문자의 하루에 맞춰 옮긴다. 한국이라면
// UTC 자정 직전에 공유한 덱이 이미 다음 날이다.
func TestInClientTZ(t *testing.T) {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}
	lateInUTC := time.Date(2026, 7, 24, 21, 9, 0, 0, time.UTC)

	decks := inClientTZ([]model.SharedDeckSummary{{SharedAt: lateInUTC}}, seoul)

	got := decks[0].SharedAt
	if y, m, d := got.Date(); y != 2026 || m != time.July || d != 25 {
		t.Errorf("SharedAt = %v, want 2026-07-25 in Seoul", got)
	}
	if !got.Equal(lateInUTC) {
		t.Error("시각 자체가 바뀌었다. 시간대 표기만 옮겨야 한다")
	}
	if label := koDate(got); label != "2026. 7. 25." {
		t.Errorf("koDate() = %q, want %q", label, "2026. 7. 25.")
	}
}

// UTC로 두면 그날의 이른 시각이 전날로 보인다. 고치기 전의 증상이다.
func TestInClientTZFixesTheOffByOneDay(t *testing.T) {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}
	sharedAt := time.Date(2026, 7, 24, 21, 9, 0, 0, time.UTC)

	if koDate(sharedAt) != "2026. 7. 24." {
		t.Fatal("이 테스트는 UTC 표기가 하루 전으로 보이는 상황을 전제한다")
	}
	if got := koDate(inClientTZ([]model.SharedDeckSummary{{SharedAt: sharedAt}}, seoul)[0].SharedAt); got == "2026. 7. 24." {
		t.Error("여전히 UTC 날짜를 보여 준다")
	}
}

func TestInClientTZLeavesAnEmptyListAlone(t *testing.T) {
	if got := inClientTZ(nil, time.UTC); got != nil {
		t.Errorf("inClientTZ(nil) = %v, want nil", got)
	}
	if got := inClientTZ([]model.SharedDeckSummary{}, time.UTC); len(got) != 0 {
		t.Errorf("inClientTZ(empty) = %v, want empty", got)
	}
}
