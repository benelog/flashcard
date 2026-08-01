package handlers

import "testing"

// 공유 슬러그는 저장소로 넘어가기 전에 모양부터 거른다. 경계 길이와 허용 문자
// 밖의 값이 어긋나면 조회 비용만 낭비된다.
func TestSlugPattern(t *testing.T) {
	valid := []string{"abcd", "AB_9-x", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} // 64자
	for _, s := range valid {
		if !slugPattern.MatchString(s) {
			t.Errorf("%q rejected, want accepted", s)
		}
	}
	invalid := []string{"", "abc", "has space", "path/slug", "dot.dot",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} // 65자
	for _, s := range invalid {
		if slugPattern.MatchString(s) {
			t.Errorf("%q accepted, want rejected", s)
		}
	}
}
