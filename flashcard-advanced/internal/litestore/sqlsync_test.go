package litestore

// 두 저장소 구현은 서로를 모른 채 같은 문장을 각자의 방언으로 유지한다(CLAUDE.md).
// 그런데 자리표시자(? / $n)만 다른 문장은 사실상 한 문장이라, 한쪽만 고치면
// 컴파일도 각자의 테스트도 잡지 못한다. schema_test.go가 두 스키마를 대조하듯
// 여기서는 그런 문장쌍을 대조한다. pgstore를 import하지 않고 소스 텍스트를 읽어
// 오므로 "어느 쪽도 상대를 링크하지 않는다"는 규칙은 그대로다.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/smartrules"
)

const pgstoreDir = "../pgstore"

// normalizeSQL은 자리표시자($n → ?)와 공백 차이를 지워 SQL의 알맹이만 남긴다.
var (
	placeholderPattern = regexp.MustCompile(`\$\d+`)
	whitespacePattern  = regexp.MustCompile(`\s+`)
)

func normalizeSQL(s string) string {
	s = placeholderPattern.ReplaceAllString(s, "?")
	return strings.TrimSpace(whitespacePattern.ReplaceAllString(s, " "))
}

// 같은 이름의 SQL 상수는 자리표시자만 빼고 같아야 한다. 같은 이름은 같은 일을
// 한다는 뜻이기 때문이다(CLAUDE.md의 대칭 규칙). 컬럼을 한쪽 select 목록에만
// 더하는 실수가 여기서 잡힌다.
func TestSharedSQLConstantsStayInSync(t *testing.T) {
	lite := stringConsts(t, ".")
	pg := stringConsts(t, pgstoreDir)

	// 방언 탓에 같은 이름이 정당하게 달라야 하는 날이 오면 여기 이유와 함께 적는다.
	excused := map[string]string{}

	shared := []string{}
	for name, liteSQL := range lite {
		pgSQL, ok := pg[name]
		if !ok {
			continue
		}
		if reason, ok := excused[name]; ok {
			t.Logf("%s: 대조 제외 (%s)", name, reason)
			continue
		}
		shared = append(shared, name)
		if normalizeSQL(liteSQL) != normalizeSQL(pgSQL) {
			t.Errorf("%s가 두 구현에서 다르다.\nlitestore: %s\npgstore:   %s",
				name, normalizeSQL(liteSQL), normalizeSQL(pgSQL))
		}
	}
	// 대조가 조용히 비어 버리면(상수 이름 변경, 파서 문제) 이 테스트는 아무것도
	// 지키지 않는다. 지금 공유 상수는 cardSelect, deckSelect, sharedDeckSelect,
	// sharedDeckBySlug, profileColumns다.
	if len(shared) < 5 {
		t.Fatalf("shared SQL constants = %v, want at least 5", shared)
	}
}

// identicalQueryFuncs는 두 구현의 SQL이 자리표시자만 빼고 같아야 하는 메서드
// 목록이다. 다음은 정당한 이유로 넣지 않는다.
//   - 시각·id를 Go에서 만들어 바인딩하는 litestore와 DB 기본값(now(),
//     gen_random_uuid())에 맡기는 pgstore가 다른 메서드: insertCard, UpdateDeck,
//     UpdateCard, RecordReview, CreateSession, FinishSession, ShareDeck,
//     GetOrCreateProfile, UpdateProfile(pg는 returning, lite는 재조회),
//     CreateDeck·ImportSharedDeck(seq 채번 방식이 다름), CreateSmartDeck
//   - 문장을 동적으로 조립해 리터럴 대조가 성립하지 않는 메서드: CardsByRule,
//     CountByRule(다만 order by 절은 TestRuleOrderingStaysInSync가 대조한다)
//   - 시간대 처리 방언이 다른 메서드: DailyStats, StatsSummary
//   - SQL이 공유 상수뿐이라 상수 대조가 대신 지키는 메서드: GetSharedDeck
//     (sharedDeckSelect + sharedDeckBySlug)
var identicalQueryFuncs = []string{
	"ListDecks", "GetDeck", "GetDeckBySlug", "DeckIDBySlug", "DeleteDeck",
	"ListCards", "GetCard", "DeleteCard", "BulkCreateCards", "DueCards", "DueCount",
	"UnshareDeck", "ListSharedDecks", "GetSharedDeckCards",
	"ListSmartDecks", "DeleteSmartDeck",
}

func TestSharedQueriesStayInSync(t *testing.T) {
	lite := sqlInFuncs(t, ".", identicalQueryFuncs)
	pg := sqlInFuncs(t, pgstoreDir, identicalQueryFuncs)

	for _, name := range identicalQueryFuncs {
		liteSQL, pgSQL := lite[name], pg[name]
		if len(liteSQL) == 0 || len(pgSQL) == 0 {
			t.Errorf("%s: SQL 문자열을 찾지 못했다. 메서드 이름이 바뀌었으면 목록도 고친다.", name)
			continue
		}
		if !slices.Equal(liteSQL, pgSQL) {
			t.Errorf("%s가 두 구현에서 다르다.\nlitestore: %q\npgstore:   %q", name, liteSQL, pgSQL)
		}
	}
}

// 규칙별 정렬 순서는 규칙의 의미인데 두 ruleQuery의 SQL 문장 안에만 있다.
// 방언이 달라 문장 전체는 대조할 수 없으므로 order by 절만 뽑아 대조한다.
// 정렬을 한쪽만 바꾸는 변경은 컴파일도 각자의 rules_test도 잡지 못하기 때문이다.
// SQLite는 asc에서 null을 이미 앞에 두므로 Postgres의 명시적 nulls first는
// 지우고 비교한다(litestore/rules.go의 Stale 주석 참고).
func TestRuleOrderingStaysInSync(t *testing.T) {
	orderByPattern := regexp.MustCompile(`order by (.+?) limit`)
	nullsFirstPattern := regexp.MustCompile(`\s+nulls first`)
	clause := func(sql string) string {
		m := orderByPattern.FindStringSubmatch(normalizeSQL(sql))
		if m == nil {
			return ""
		}
		return nullsFirstPattern.ReplaceAllString(m[1], "")
	}

	// pgstore 쪽은 소스의 문자열 리터럴에서 읽는다. 어느 규칙의 문장인지는
	// 그 규칙에만 있는 조건으로 알아본다.
	pgClauses := map[smartrules.RuleType]string{}
	for _, lit := range sqlInFuncs(t, pgstoreDir, []string{"ruleQuery"})["ruleQuery"] {
		c := clause(lit)
		if c == "" {
			continue
		}
		var rt smartrules.RuleType
		switch {
		case strings.Contains(lit, "error_rate"):
			rt = smartrules.HighError
		case strings.Contains(lit, "last_reviewed_at"):
			rt = smartrules.Stale
		case strings.Contains(lit, "tags &&"):
			rt = smartrules.Tag
		case strings.Contains(lit, "created_at >="):
			rt = smartrules.Recent
		default:
			t.Errorf("pgstore ruleQuery에서 어느 규칙인지 알 수 없는 문장: %q", lit)
			continue
		}
		pgClauses[rt] = c
	}

	// litestore 쪽은 진짜 ruleQuery를 불러 확인한다. 새 규칙 종류를 더하면
	// 여기에도 견본을 더해야 두 구현의 정렬이 계속 묶인다.
	samples := map[smartrules.RuleType]smartrules.Rule{
		smartrules.HighError: {Type: smartrules.HighError},
		smartrules.Stale:     {Type: smartrules.Stale},
		smartrules.Tag:       {Type: smartrules.Tag, Tags: []string{"t"}},
		smartrules.Recent:    {Type: smartrules.Recent},
	}
	if len(pgClauses) != len(samples) {
		t.Errorf("pgstore ruleQuery의 규칙 수 = %d, litestore 견본 수 = %d — 한쪽에만 규칙이 추가됐다",
			len(pgClauses), len(samples))
	}
	for rt, rule := range samples {
		sql, _ := ruleQuery(rule, time.Unix(0, 0))
		got := clause(sql)
		want, ok := pgClauses[rt]
		if !ok {
			t.Errorf("%s: pgstore ruleQuery에서 order by를 찾지 못했다", rt)
			continue
		}
		if got != want {
			t.Errorf("%s 규칙의 정렬이 두 구현에서 다르다.\nlitestore: %q\npgstore:   %q", rt, got, want)
		}
	}
}

// parseSources는 패키지의 테스트가 아닌 Go 파일을 파일 이름 순으로 파싱한다.
func parseSources(t *testing.T, dir string) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := []*ast.File{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatalf("no Go sources in %s", dir)
	}
	return files
}

// stringConsts는 패키지 최상위의 문자열 상수를 이름별로 모은다.
func stringConsts(t *testing.T, dir string) map[string]string {
	t.Helper()
	consts := map[string]string{}
	for _, f := range parseSources(t, dir) {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						continue
					}
					consts[name.Name] = value
				}
			}
		}
	}
	return consts
}

// sqlLiteralPattern은 문자열 리터럴이 SQL(또는 상수 뒤에 이어 붙는 조각)인지
// 가려낸다. 에러 메시지 같은 일반 문자열이 걸리지 않도록 키워드 단위로 본다.
var sqlLiteralPattern = regexp.MustCompile(`(?i)\b(select|insert|update|delete|where|order by)\b`)

// sqlInFuncs는 이름을 지정한 각 함수의 SQL 문자열 리터럴을 소스 순서대로,
// 정규화해 돌려준다.
func sqlInFuncs(t *testing.T, dir string, names []string) map[string][]string {
	t.Helper()
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	found := map[string][]string{}
	for _, f := range parseSources(t, dir) {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !wanted[fn.Name.Name] {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil || !sqlLiteralPattern.MatchString(value) {
					return true
				}
				found[fn.Name.Name] = append(found[fn.Name.Name], normalizeSQL(value))
				return true
			})
		}
	}
	return found
}
