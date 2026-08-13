package litestore

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// schema.sql은 internal/db/migrations의 Postgres 스키마를 손으로 옮긴 포팅본이다.
// 둘이 어긋나도 컴파일 오류도 테스트 실패도 나지 않는다: 로컬에서만 되거나 배포
// 에서만 되는 상태로 조용히 갈라질 뿐이다. 여기 있는 테스트가 프로젝트 지침의
// "같은 커밋에서 함께 고쳐야 한다"는 문장을 실제 게이트로 바꾼다.
//
// 마이그레이션 SQL을 완전히 해석하지는 않는다. 실제로 잊기 쉬운 세 가지, 곧 새
// 테이블과 새 뷰와 새 열만 본다.

const migrationsDir = "../db/migrations"

// migrationFiles는 up 마이그레이션을 golang-migrate가 적용하는 순서로 나열한다.
func migrationFiles(t *testing.T) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no up migrations found in %s (%v)", migrationsDir, err)
	}
	sort.Strings(names)
	return names
}

func migrationSQL(t *testing.T) string {
	t.Helper()
	var all strings.Builder
	for _, name := range migrationFiles(t) {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		all.Write(b)
		all.WriteString("\n")
	}
	return all.String()
}

// names는 모든 매치의 첫 캡처 그룹을 중복 없이 모은다.
func names(pattern, sql string) []string {
	found := map[string]bool{}
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(sql, -1) {
		found[strings.ToLower(m[1])] = true
	}
	out := make([]string, 0, len(found))
	for n := range found {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// sqliteNames는 적용된 SQLite 스키마에 실제로 무엇이 만들어졌는지 물어본다.
func sqliteNames(t *testing.T, kind string) []string {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "schema.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer s.Close()

	rows, err := s.db.Query(
		`select name from sqlite_master where type = ? and name not like 'sqlite_%' order by name`, kind)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// sqliteColumns는 모든 테이블의 모든 열을 "table.column" 형태로 나열한다.
func sqliteColumns(t *testing.T) map[string]bool {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "schema.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer s.Close()

	rows, err := s.db.Query(
		`select m.name, p.name from sqlite_master m
		 join pragma_table_info(m.name) p
		 where m.type = 'table' and m.name not like 'sqlite_%'`)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatal(err)
		}
		columns[table+"."+column] = true
	}
	return columns
}

func TestSQLitePortHasEveryTable(t *testing.T) {
	want := names(`(?i)create\s+table\s+(?:if\s+not\s+exists\s+)?([a-z_]+)`, migrationSQL(t))
	got := sqliteNames(t, "table")

	for _, table := range want {
		if !slices.Contains(got, table) {
			t.Errorf("table %q exists in the Postgres migrations but not in schema.sql", table)
		}
	}
	for _, table := range got {
		if !slices.Contains(want, table) {
			t.Errorf("table %q exists in schema.sql but not in the Postgres migrations", table)
		}
	}
}

func TestSQLitePortHasEveryView(t *testing.T) {
	want := names(`(?i)create\s+view\s+(?:if\s+not\s+exists\s+)?([a-z_]+)`, migrationSQL(t))
	got := sqliteNames(t, "view")

	for _, view := range want {
		if !slices.Contains(got, view) {
			t.Errorf("view %q exists in the Postgres migrations but not in schema.sql", view)
		}
	}
}

// 새 마이그레이션이 열을 하나 더하고 schema.sql을 잊는 것이 가장 흔한 사고다.
// 초기 create table의 열들은 litestore의 다른 테스트가 실제 쿼리로 훑으므로,
// 여기서는 나중에 더해지거나 이름이 바뀐 열만 본다.
func TestSQLitePortHasColumnsAddedByMigrations(t *testing.T) {
	sql := migrationSQL(t)
	columns := sqliteColumns(t)

	added := regexp.MustCompile(`(?is)alter\s+table\s+([a-z_]+)(.*?);`)
	addColumn := regexp.MustCompile(`(?i)add\s+column\s+(?:if\s+not\s+exists\s+)?([a-z_]+)`)
	renameTo := regexp.MustCompile(`(?i)rename\s+column\s+[a-z_]+\s+to\s+([a-z_]+)`)

	checked := 0
	for _, stmt := range added.FindAllStringSubmatch(sql, -1) {
		table := strings.ToLower(stmt[1])
		for _, pattern := range []*regexp.Regexp{addColumn, renameTo} {
			for _, m := range pattern.FindAllStringSubmatch(stmt[2], -1) {
				column := strings.ToLower(m[1])
				checked++
				if !columns[table+"."+column] {
					t.Errorf("migrations add %s.%s, but schema.sql has no such column", table, column)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no altered columns found; the migration parser needs updating")
	}
}

// 이 스키마보다 앞서 만들어진 로컬 DB 파일은 create ... if not exists가
// 건너뛰므로, 나중에 추가된 열은 Open의 ensureColumn이 보정한다. 그 보정이
// 실제로 동작하는지 옛 파일을 흉내 내어 확인한다.
func TestOpenAddsColumnsToOldFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// story 열이 생기기 전의 파일처럼 만든다.
	if _, err := s.db.Exec(`alter table decks drop column story`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer s.Close()
	var n int
	if err := s.db.QueryRow(
		`select count(*) from pragma_table_info('decks') where name = 'story'`).Scan(&n); err != nil {
		t.Fatalf("check column: %v", err)
	}
	if n != 1 {
		t.Error("reopening an old file did not add the decks.story column")
	}
}

// schema.sql은 어느 마이그레이션까지 반영했는지를 첫머리에 적어 둔다. 새
// 마이그레이션을 넣고 이 표식을 갱신하지 않으면 여기서 걸린다.
func TestSQLitePortRecordsLatestMigration(t *testing.T) {
	files := migrationFiles(t)
	latest := strings.TrimSuffix(filepath.Base(files[len(files)-1]), ".up.sql")

	marker := regexp.MustCompile(`(?i)ported through:\s*(\S+)`).FindStringSubmatch(schemaSQL)
	if marker == nil {
		t.Fatalf("schema.sql has no %q line; add one naming the last migration it ports", "Ported through:")
	}
	if marker[1] != latest {
		t.Errorf("schema.sql says it ports through %q, but the latest migration is %q.\n"+
			"새 마이그레이션을 SQLite로 옮긴 뒤 이 표식도 함께 고친다.", marker[1], latest)
	}
}
