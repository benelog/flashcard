package web

import (
	"testing"
	"time"
)

// "오늘 복습"의 만기 상한은 방문자 시간대의 그날 마지막 순간이다. 시각을 주입
// 받으므로 자정 경계와 시간대 변환을 시계 없이 검증한다.
func TestEndOfToday(t *testing.T) {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}

	// UTC 7/24 16:00 = 서울 7/25 01:00. 서울 방문자의 "오늘"은 이미 25일이다.
	now := time.Date(2026, 7, 24, 16, 0, 0, 0, time.UTC)
	got := endOfToday(now, seoul)
	want := time.Date(2026, 7, 25, 23, 59, 59, 0, seoul)
	if !got.Equal(want) {
		t.Errorf("endOfToday(%v, Seoul) = %v, want %v", now, got, want)
	}

	// 같은 순간이라도 UTC 방문자의 하루는 아직 24일이다.
	got = endOfToday(now, time.UTC)
	want = time.Date(2026, 7, 24, 23, 59, 59, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("endOfToday(%v, UTC) = %v, want %v", now, got, want)
	}
}
