package study

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// 말해서 답하기와 따라 말하기의 채점. 브라우저의 음성 인식(app.js)은 들은 말을
// 글자로 바꿔 보내기만 하고, 그것이 정답과 맞는지는 여기서 정한다. 화면이 몇
// 개든 판정 기준은 한 벌이어야 하고, 브라우저 없이 단위 테스트할 수 있어야 하기
// 때문이다.
//
// 음성 인식은 발음을 채점하지 않는다. 들린 말을 가장 그럴듯한 문장으로 받아
// 적을 뿐이라, 여기서 보는 것은 "정답 문장으로 알아들었는가"다.

// SpokenWord는 정답을 이루는 단어 하나와, 그 단어가 들렸는지다.
type SpokenWord struct {
	Text  string
	Heard bool
}

// SpeechMatch는 말한 것을 정답과 견준 결과다.
type SpeechMatch struct {
	Correct bool
	Score   int    // 정답 단어 중 들린 비율(%)
	Heard   string // 인식 후보 중 정답에 가장 가까웠던 것
	Words   []SpokenWord
}

// 정답으로 치는 기준. 비율만 보면 "It's my last day"가 "It's my first day"의
// 80%라 맞은 것이 되어 버린다. 그래서 뜻을 싣는 단어는 빠짐없이 들려야 하고
// (긴 문장만 10단어에 하나꼴로 봐준다), 관사·be동사·please처럼 인식기가 흘리기
// 쉽고 빠져도 뜻이 남는 말(lightWords)만 놓쳐도 된다. 또 들린 말의 60% 이상이
// 정답 단어여야 한다(엉뚱한 말을 길게 늘어놓아 우연히 맞는 것을 막는다).
const (
	contentMissEvery = 10
	minPrecision     = 0.6
)

var lightWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "am": true, "are": true,
	"please": true, "so": true, "well": true, "oh": true, "just": true,
	"yeah": true, "okay": true, "um": true, "uh": true, "hey": true,
	"really": true, "very": true, "actually": true,
}

// MatchSpeech는 인식 후보들(heard) 가운데 정답에 가장 가까운 것으로 채점한다.
// 정답에 " / "로 나뉜 표현이 여럿이면 그중 하나만 맞아도 된다. 괄호 안은
// 보충 설명으로 보고 말하지 않아도 된다.
func MatchSpeech(expected string, heard []string) SpeechMatch {
	if len(heard) == 0 {
		heard = []string{""} // 들은 것이 없어도 정답 단어 표시는 채운다
	}
	var best SpeechMatch
	first := true
	for _, alt := range strings.Split(expected, " / ") {
		if strings.TrimSpace(alt) == "" {
			continue
		}
		for _, h := range heard {
			m := matchOne(strings.TrimSpace(alt), strings.TrimSpace(h))
			if first || better(m, best) {
				best, first = m, false
			}
		}
	}
	return best
}

// missAllowance는 놓쳐도 되는 내용어 수다. 한국어는 띄어쓰기가 흔들리는 글자
// 단위라 다섯 글자에 하나꼴로 봐준다(조사 하나쯤은 인식이 달라도 된다).
func missAllowance(tokens int, korean bool) int {
	if korean {
		return tokens / 5
	}
	return tokens / contentMissEvery
}

func better(a, b SpeechMatch) bool {
	if a.Correct != b.Correct {
		return a.Correct
	}
	return a.Score > b.Score
}

func matchOne(expected, heard string) SpeechMatch {
	korean := hasHangul(expected)
	m := SpeechMatch{Heard: heard}

	// 정답을 화면에 보일 단어로 나누고, 단어마다 비교용 토큰을 만든다.
	type word struct {
		text   string
		tokens []string
		from   int // exp에서 이 단어의 토큰이 시작하는 자리
	}
	var words []word
	var exp []string
	optional := false
	for _, w := range strings.Fields(expected) {
		if strings.HasPrefix(w, "(") {
			optional = true
		}
		var tokens []string
		if !optional {
			tokens = speechTokens(w, korean)
		}
		if strings.HasSuffix(w, ")") {
			optional = false
		}
		words = append(words, word{text: w, tokens: tokens, from: len(exp)})
		exp = append(exp, tokens...)
	}
	var got []string
	for _, w := range strings.Fields(heard) {
		got = append(got, speechTokens(w, korean)...)
	}

	matched := alignTokens(exp, got)
	hits, contentMisses := 0, 0
	for i, ok := range matched {
		switch {
		case ok:
			hits++
		case !korean && !lightWords[exp[i]]:
			contentMisses++
		case korean:
			contentMisses++ // 한국어는 글자 단위라 가벼운 말을 가려낼 수 없다
		}
	}
	for _, w := range words {
		heardAll := true
		for i := range w.tokens {
			heardAll = heardAll && matched[w.from+i]
		}
		m.Words = append(m.Words, SpokenWord{Text: w.text, Heard: heardAll})
	}
	if len(exp) == 0 || len(got) == 0 {
		return m
	}
	precision := float64(hits) / float64(len(got))
	m.Score = int(math.Round(float64(hits) / float64(len(exp)) * 100))
	m.Correct = contentMisses <= missAllowance(len(exp), korean) && precision >= minPrecision
	return m
}

// alignTokens는 두 토큰 열의 최장 공통 부분열을 찾아, 정답 토큰마다 짝이
// 있는지 돌려준다. 순서까지 맞아야 들린 것으로 친다.
func alignTokens(exp, got []string) []bool {
	n, k := len(exp), len(got)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, k+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := k - 1; j >= 0; j-- {
			if sameToken(exp[i], got[j]) {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	matched := make([]bool, n)
	for i, j := 0, 0; i < n && j < k; {
		switch {
		case sameToken(exp[i], got[j]) && lcs[i][j] == lcs[i+1][j+1]+1:
			matched[i] = true
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			i++
		default:
			j++
		}
	}
	return matched
}

// homophones는 인식기가 같은 소리를 다른 글자로 적는 흔한 짝이다. 말한
// 사람은 틀리지 않았으므로 같은 것으로 본다.
var homophones = map[string]string{
	"too": "to", "2": "to",
	"4":     "for",
	"their": "there",
	"ok":    "okay",
	"em":    "them",
}

func sameToken(a, b string) bool {
	if a == b {
		return true
	}
	if h, ok := homophones[a]; ok {
		a = h
	}
	if h, ok := homophones[b]; ok {
		b = h
	}
	if a == b {
		return true
	}
	// favorite/favourite처럼 철자만 조금 다른 긴 단어.
	return len(a) >= 5 && len(b) >= 5 && editDistance(a, b) <= 1
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

// speechTokens는 단어 하나를 비교용 토큰으로 바꾼다. 한국어는 띄어쓰기가
// 인식 결과마다 달라 글자 단위로, 영어는 단어 단위로 견준다. 영어는 말로는
// 구별되지 않는 차이를 지운다: 대소문자, 문장 부호, 축약형(it's = it is),
// 숫자를 숫자로 적었는지 말로 적었는지(two fifty = $2.50).
func speechTokens(word string, korean bool) []string {
	word = strings.ToLower(strings.NewReplacer("’", "'", "‘", "'").Replace(word))
	if korean {
		var out []string
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				out = append(out, string(r))
			}
		}
		return out
	}

	// 숫자 사이의 . : , 와 하이픈은 말할 때 끊기는 자리라 토큰을 나눈다.
	parts := strings.FieldsFunc(word, func(r rune) bool {
		return r == '-' || r == '.' || r == ':' || r == ',' || r == '/'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimFunc(p, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if p == "" {
			continue
		}
		if expanded, ok := contractions[p]; ok {
			out = append(out, expanded...)
			continue
		}
		if strings.HasSuffix(p, "n't") {
			out = append(out, strings.TrimSuffix(p, "n't"), "not")
			continue
		}
		if base, suffix, ok := strings.Cut(p, "'"); ok {
			if full, known := contractionSuffixes[suffix]; known {
				out = append(out, base, full)
				continue
			}
		}
		out = append(out, numberTokens(p)...)
	}
	return out
}

var contractions = map[string][]string{
	"i'm": {"i", "am"}, "it's": {"it", "is"}, "that's": {"that", "is"},
	"what's": {"what", "is"}, "there's": {"there", "is"}, "here's": {"here", "is"},
	"he's": {"he", "is"}, "she's": {"she", "is"}, "who's": {"who", "is"},
	"where's": {"where", "is"}, "how's": {"how", "is"}, "let's": {"let", "us"},
	"can't": {"can", "not"}, "won't": {"will", "not"}, "cannot": {"can", "not"},
	"alright": {"all", "right"}, "gonna": {"going", "to"}, "wanna": {"want", "to"},
}

var contractionSuffixes = map[string]string{
	"re": "are", "ll": "will", "ve": "have", "d": "would", "m": "am",
}

var numberWords = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19, "twenty": 20, "thirty": 30, "forty": 40,
	"fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

// numberTokens는 수를 말로 쓴 것과 숫자로 쓴 것을 같은 모양으로 맞춘다.
// "twenty five"는 두 토큰이므로 "25"도 "20", "5" 두 토큰으로 나눈다.
func numberTokens(p string) []string {
	if n, ok := numberWords[p]; ok {
		return []string{strconv.Itoa(n)}
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 21 || n > 99 || n%10 == 0 {
		return []string{p}
	}
	return []string{strconv.Itoa(n / 10 * 10), strconv.Itoa(n % 10)}
}

func hasHangul(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}
