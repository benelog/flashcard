package model

import "time"

// Location resolves an IANA timezone name the visitor's browser reported.
// 방문자가 어디 있는지는 HTML 화면은 쿠키로, JSON API는 질의 문자열로 알려
// 오는데, 값을 해석하는 규칙은 하나여야 통계의 "오늘"이 두 곳에서 같아진다.
// 비어 있거나 알아볼 수 없는 이름이면 UTC로 떨어진다. IANA 데이터베이스로
// 검증된 이름만 돌려주므로 SQL의 시간대 인자로 그대로 써도 안전하다.
func Location(tz string) (string, *time.Location) {
	if tz == "" {
		return "UTC", time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "UTC", time.UTC
	}
	return tz, loc
}
