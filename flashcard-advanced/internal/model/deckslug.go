package model

import (
	"fmt"
	"strings"
)

// 이 파일의 슬러그 함수는 대문자 이름으로 공개한다. 다른 Store 구현
// (internal/litestore)이 같은 규칙으로 슬러그를 만들어야 하기 때문이다. 규칙을
// 양쪽에 베껴 두면 로컬과 운영의 덱 주소가 달라진다.
//
// 덱 URL slug는 항상 Base36 4글자다. Base36(0-9, a-z)은 대소문자를 가리지
// 않아 사용자가 대소문자 걱정 없이 입력할 수 있다. seq 열을 그대로 내보내면
// 순번 카운터("0001", "0002", …)가 드러나므로, 36^4 슬러그 공간의 곱셈 순열을
// 한 번 거친다. 연달아 만든 덱이 흩어진, 규칙이 안 보이는 slug를 받는다.
// 이 사상은 전단사라 slug에서 seq를 되찾아 조회할 수 있다. UUID id는 밖으로
// 내지 않는다.
// tag::slug-constants[]
const slugAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// slugLen은 slug의 고정 길이다. slugSpace = 36^slugLen은 서로 다른 slug의
// 수이자 순열이 다룰 수 있는 최대 seq다. seq는 모든 사용자가 공유하는 전역
// 시퀀스라, slugLen=4는 전체 덱 수를 slugSpace-1(1,679,615)로 제한한다. 그
// 뒤에 만든 덱은 빈 slug를 받는다. 한도를 늘리려면 slugLen을 넓히거나 seq를
// 사용자별 번호로 바꾼다.
const slugLen = 4
const slugSpace = 36 * 36 * 36 * 36

// slugMul은 slugSpace와 서로소다(36의 소인수인 2로도 3으로도 나뉘지 않는다).
// 그래서 slugSpace를 법으로 한 곱셈이 가역이 되고, slugMulInv가 그것을 되돌린다.
const slugMul = 1038007

var slugMulInv = modInverse(slugMul, slugSpace)

// end::slug-constants[]

// tag::encode[]
func EncodeDeckSlug(seq int64) string {
	if seq <= 0 || seq >= slugSpace {
		return ""
	}
	n := (seq * slugMul) % slugSpace
	var buf [slugLen]byte
	for i := slugLen - 1; i >= 0; i-- {
		buf[i] = slugAlphabet[n%36]
		n /= 36
	}
	return string(buf[:])
}

// end::encode[]

func DecodeDeckSlug(s string) (int64, error) {
	if len(s) != slugLen {
		return 0, fmt.Errorf("invalid deck slug %q", s)
	}
	s = strings.ToLower(s) // slug는 대소문자를 가리지 않는다
	var n int64
	for i := 0; i < len(s); i++ {
		v := strings.IndexByte(slugAlphabet, s[i])
		if v < 0 {
			return 0, fmt.Errorf("invalid deck slug %q", s)
		}
		n = n*36 + int64(v)
	}
	seq := (n * slugMulInv) % slugSpace
	if seq <= 0 {
		return 0, fmt.Errorf("invalid deck slug %q", s)
	}
	return seq, nil
}

// modInverse는 확장 유클리드 알고리즘으로 a^-1 mod m을 구한다. gcd(a, m) == 1을
// 전제한다. 시작할 때 slugMul에서 slugMulInv를 만드는 데 한 번 쓴다.
func modInverse(a, m int64) int64 {
	t, newT := int64(0), int64(1)
	r, newR := m, a
	for newR != 0 {
		q := r / newR
		t, newT = newT, t-q*newT
		r, newR = newR, r-q*newR
	}
	if t < 0 {
		t += m
	}
	return t
}
