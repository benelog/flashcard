package model

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"
)

type ShareInfo struct {
	ShareSlug string    `json:"shareSlug"`
	SharedAt  time.Time `json:"sharedAt"`
}

type SharedDeckSummary struct {
	ShareSlug   string    `json:"shareSlug"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CardCount   int       `json:"cardCount"`
	OwnerName   *string   `json:"ownerName"`
	SharedAt    time.Time `json:"sharedAt"`
	IsMine      bool      `json:"isMine"`
}

// SharedCard는 소유자가 아닌 사람에게 내보내는 카드다. 내용만 담고 id와 SRS
// 상태는 내보내지 않는다.
type SharedCard struct {
	Text     string   `json:"text"`
	Meaning  string   `json:"meaning"`
	CardType string   `json:"cardType"`
	Tags     []string `json:"tags"`
	Phonetic *string  `json:"phonetic"`
	Example  *string  `json:"example"`
	Notes    *string  `json:"notes"`
}

// 공유 slug는 비밀이 아니다. /shared 갤러리가 공유된 덱을 모두 공개로 나열하니
// 비밀을 위한 엔트로피는 필요 없고, 전역에서 유일하기만 하면 된다. 덱 slug와
// 같은 대소문자 무시 알파벳을 쓰는 무작위 Base36 5글자 토큰이며, 드물게
// 충돌하면 ShareDeck이 unique 인덱스에 걸린 것을 보고 다시 시도한다.
const shareSlugLen = 5

var shareSlugSpace = new(big.Int).Exp(big.NewInt(36), big.NewInt(shareSlugLen), nil)

func NewShareSlug() string {
	n, err := rand.Int(rand.Reader, shareSlugSpace)
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s := n.Text(36) // 소문자 Base36
	if len(s) < shareSlugLen {
		s = strings.Repeat("0", shareSlugLen-len(s)) + s
	}
	return s
}
