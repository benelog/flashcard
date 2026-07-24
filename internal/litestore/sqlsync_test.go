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
	// 지키지 않는다. 지금 공유 상수는 cardSelect, deckSelect, sharedDeckSelect다.
	if len(shared) < 3 {
		t.Fatalf("shared SQL constants = %v, want at least 3", shared)
	}
}

// identicalQueryFuncs는 두 구현의 SQL이 자리표시자만 빼고 같아야 하는 메서드
// 목록이다. 시각·id를 Go에서 만들어 바인딩하는 litestore와 DB 기본값(now(),
// gen_random_uuid())에 맡기는 pgstore가 정당하게 다른 메서드(insertCard,
// UpdateDeck, UpdateCard, RecordReview, CreateSession, FinishSession,
// ShareDeck, GetOrCreateProfile, ImportSharedDeck, CardsByRule)는 넣지 않는다.
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

// parseSources parses the package's non-test Go files, in file-name order.
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

// stringConsts collects the package's top-level string constants by name.
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

// sqlInFuncs returns each named function's SQL string literals in source
// order, normalized.
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
