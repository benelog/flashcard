package model

import (
	"testing"

	"github.com/google/uuid"
)

// 스마트 규칙이 정한 순서(오답률 높은 순 등)는 ID 목록에만 남고, 카드 본문을
// 가져오는 두 번째 조회는 그 순서를 지켜 주지 않는다. 저장소 구현 둘이 이
// 함수로 순서를 되살리므로 계약을 여기서 고정한다.
func TestSortCardsByIDOrder(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	cards := []Card{{ID: c}, {ID: a}, {ID: b}}

	got := SortCardsByIDOrder(cards, []uuid.UUID{a, b, c})

	if len(got) != 3 || got[0].ID != a || got[1].ID != b || got[2].ID != c {
		t.Errorf("cards came back out of id order: %v", ids(got))
	}
}

// ids에는 있지만 두 번째 조회가 찾지 못한 카드(그 사이 삭제된 경우)는 조용히
// 빠지고, 나머지 순서는 유지된다.
func TestSortCardsByIDOrderDropsUnresolvedIDs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	deleted := uuid.New()

	got := SortCardsByIDOrder([]Card{{ID: b}, {ID: a}}, []uuid.UUID{a, deleted, b})

	if len(got) != 2 || got[0].ID != a || got[1].ID != b {
		t.Errorf("got %v, want [a b] with the deleted id dropped", ids(got))
	}
}

// ids가 고르지 않은 카드는 결과에 나타나지 않는다: 순서의 근거가 없기 때문이다.
func TestSortCardsByIDOrderIgnoresCardsOutsideTheList(t *testing.T) {
	a, stray := uuid.New(), uuid.New()

	got := SortCardsByIDOrder([]Card{{ID: a}, {ID: stray}}, []uuid.UUID{a})

	if len(got) != 1 || got[0].ID != a {
		t.Errorf("got %v, want only the listed card", ids(got))
	}
}

// 빈 ID 목록이면 nil이 아니라 빈 슬라이스다: JSON으로 나가면 null 대신 []가 된다.
func TestSortCardsByIDOrderEmptyIDs(t *testing.T) {
	got := SortCardsByIDOrder([]Card{{ID: uuid.New()}}, nil)

	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want an empty (non-nil) slice", got)
	}
}

func ids(cards []Card) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.ID)
	}
	return out
}
