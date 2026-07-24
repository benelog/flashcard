package model

import (
	"testing"
	"time"
)

// 방문자가 알려 온 시간대 이름 하나로 통계의 "오늘"이 정해진다. 알아볼 수 없는
// 값에 통계 화면 전체가 실패하느니 UTC로 떨어지는 편이 낫다.
func TestLocation(t *testing.T) {
	tests := []struct {
		name   string
		given  string
		wantTZ string
	}{
		{"보고하지 않음", "", "UTC"},
		{"정상 이름", "Asia/Seoul", "Asia/Seoul"},
		{"없는 이름", "Mars/Olympus", "UTC"},
		{"장난스러운 값", "'; drop table--", "UTC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tz, loc := Location(tt.given)
			if tz != tt.wantTZ {
				t.Errorf("Location(%q) tz = %q, want %q", tt.given, tz, tt.wantTZ)
			}
			if loc == nil {
				t.Fatalf("Location(%q) returned a nil location", tt.given)
			}
			if loc.String() != tt.wantTZ {
				t.Errorf("Location(%q) loc = %v, want %v", tt.given, loc, tt.wantTZ)
			}
		})
	}
}

// 돌려주는 이름과 Location은 같은 곳을 가리켜야 한다: 하나는 SQL의 시간대
// 인자로, 다른 하나는 Go 쪽 날짜 계산으로 가기 때문이다.
func TestLocationNameMatchesOffset(t *testing.T) {
	tz, loc := Location("Asia/Seoul")

	parsed, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatalf("returned tz %q does not load: %v", tz, err)
	}
	noon := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	if noon.In(parsed).Format(time.RFC3339) != noon.In(loc).Format(time.RFC3339) {
		t.Error("tz name and location disagree")
	}
}
