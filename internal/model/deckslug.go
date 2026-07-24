package model

import (
	"fmt"
	"strings"
)

// 이 파일의 슬러그 함수는 대문자 이름으로 공개한다. 다른 Store 구현
// (internal/litestore)이 같은 규칙으로 슬러그를 만들어야 하기 때문이다. 규칙을
// 양쪽에 베껴 두면 로컬과 운영의 덱 주소가 달라진다.
//
// Deck URL slugs are always exactly 4 Base36 characters. Base36 (0-9, a-z) is
// case-insensitive, so users can type a slug without worrying about case.
// Exposing the raw seq column directly would leak a sequential counter ("0001",
// "0002", ...), so seq is first run through a multiplicative permutation over
// the 36^4 slug space: consecutive decks land on scattered, non-obvious slugs.
// The mapping is a bijection, so a slug still decodes back to its seq for
// lookups. The UUID id stays internal.
// tag::slug-constants[]
const slugAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// slugLen is the fixed slug width; slugSpace = 36^slugLen is the number of
// distinct slugs and the largest seq the permutation can address. seq is a
// GLOBAL identity sequence, so slugLen=4 caps the total number of decks across
// all users at slugSpace-1 (1,679,615); a deck created past that would get an
// empty slug. Widen slugLen (or switch seq to per-user numbering) to lift it.
const slugLen = 4
const slugSpace = 36 * 36 * 36 * 36

// slugMul is coprime to slugSpace (not divisible by 2 or 3, the prime factors
// of 36), which makes multiplication by it modulo slugSpace invertible.
// slugMulInv reverses it.
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
	s = strings.ToLower(s) // slugs are case-insensitive
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

// modInverse returns a^-1 mod m via the extended Euclidean algorithm, assuming
// gcd(a, m) == 1. Used once at startup to derive slugMulInv from slugMul.
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
