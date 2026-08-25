package tui

import (
	"strings"
	"testing"
)

// 펼친 메뉴가 메뉴 바의 제목 아래에서 시작하는지 본다.
func TestMenuViewDropdown(t *testing.T) {
	m := newMenu(nil, "")
	m.bar, m.open, m.sel = 1, true, 0 // 덱 메뉴

	view := m.View()
	t.Log("\n" + view)

	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[0], "프로그램") || !strings.Contains(lines[0], "학습") {
		t.Errorf("메뉴 바가 없다: %q", lines[0])
	}
	if !strings.Contains(view, "카드 보기…") {
		t.Error("펼친 메뉴의 항목이 안 보인다")
	}
	if got := indentOf(lines[1]); got != m.dropOffset() {
		t.Errorf("펼친 메뉴 위치가 %d칸, 메뉴 제목은 %d칸", got, m.dropOffset())
	}
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
