package model

import "strings"

// 선택 필드(nullable 열)의 두 얼굴을 오가는 변환이다. Card의 phonetic·example·
// notes와 Deck의 description이 모두 *string이라, 사람이 채우는 입력(웹 폼, CSV)과
// 사람에게 보이는 출력(템플릿, CSV)이 같은 규칙을 따르도록 여기 모아 둔다.

// NilIfBlank는 비어 있는 선택 필드를 빈 문자열이 아니라 NULL로 저장되게 한다.
// "적지 않았다"와 "빈 값을 적었다"가 섞이지 않게 하기 위함이다.
func NilIfBlank(v string) *string {
	if v = strings.TrimSpace(v); v == "" {
		return nil
	}
	return &v
}

// OrEmpty는 선택 필드를 출력한다. 값이 없으면 빈칸이다. 템플릿에서 nil 포인터를
// 그대로 쓰면 "<nil>"이 찍히기 때문에 반드시 거쳐야 한다.
func OrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
