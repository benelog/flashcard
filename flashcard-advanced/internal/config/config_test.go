package config

import "testing"

// 대시보드 환경 변수 칸에 붙여 넣을 때 딸려 온 줄바꿈이 설정 값에 살아남으면 안
// 된다. net/http는 제어 문자가 든 헤더 값을 거부하므로, 다듬지 않은 키는 GoTrue
// 요청 전부를 깨뜨린다(2026-07-11 운영 장애, fix-auth.md 참고).
func TestLoadTrimsWhitespace(t *testing.T) {
	t.Setenv("DATABASE_URL", " postgres://example/db \n")
	t.Setenv("SUPABASE_URL", "https://ref.supabase.co/\n")
	t.Setenv("SUPABASE_ANON_KEY", "sb_publishable_key\n")
	t.Setenv("SUPABASE_JWKS_URL", "")
	t.Setenv("SUPABASE_JWT_SECRET", "")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseURL != "postgres://example/db" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.SupabaseURL != "https://ref.supabase.co" {
		t.Errorf("SupabaseURL = %q", cfg.SupabaseURL)
	}
	if cfg.SupabaseAnonKey != "sb_publishable_key" {
		t.Errorf("SupabaseAnonKey = %q", cfg.SupabaseAnonKey)
	}
}
