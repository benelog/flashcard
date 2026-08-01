package litestore

import (
	"testing"
	"time"
)

// UTC 15시는 서울의 자정이다. 그 경계 앞뒤의 리뷰가 서울 기준으로 다른 날에
// 담기는지 확인한다. DB에 담긴 시각은 전부 UTC이므로 이 변환이 틀리면 자정
// 직후의 학습이 어제로 집계된다.
func TestBucketDailySplitsOnLocalMidnight(t *testing.T) {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}
	moments := []reviewMoment{
		{At: time.Date(2026, 7, 24, 14, 59, 0, 0, time.UTC), Correct: true},
		{At: time.Date(2026, 7, 24, 15, 1, 0, 0, time.UTC), Correct: false},
		{At: time.Date(2026, 7, 24, 15, 2, 0, 0, time.UTC), Correct: true},
	}

	got := bucketDaily(moments, seoul)

	if len(got) != 2 {
		t.Fatalf("buckets = %+v, want two local days", got)
	}
	if got[0].Date != "2026-07-24" || got[0].Total != 1 || got[0].Correct != 1 {
		t.Errorf("first day = %+v", got[0])
	}
	if got[1].Date != "2026-07-25" || got[1].Total != 2 || got[1].Correct != 1 {
		t.Errorf("second day = %+v", got[1])
	}
}

func TestBucketDailyEmpty(t *testing.T) {
	got := bucketDaily(nil, time.UTC)
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want an empty (non-nil) slice", got)
	}
}
