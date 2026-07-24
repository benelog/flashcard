package config

import (
	"fmt"
	"os"
	"strings"
)

// env는 환경 변수를 둘레 공백을 걷어 내고 읽는다. 대시보드 입력 칸에 붙여
// 넣을 때 딸려 온 줄바꿈이 HTTP 헤더까지 살아남으면 net/http가 값을 거부해
// 요청이 아예 나가지 못한다.
func env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

type Config struct {
	Driver          string // "postgres"(배포) 또는 "sqlite"(로컬 모드)
	DatabaseURL     string
	SQLitePath      string
	AuthMode        string // "supabase"(JWT 검증) 또는 "local"(고정 사용자)
	SupabaseURL     string // https://<ref>.supabase.co — 웹 로그인(GoTrue) 기본 URL
	SupabaseAnonKey string // 서버 쪽 OAuth 흐름에 쓰는 GoTrue apikey
	JWKSURL         string
	JWTSecret       string // 옛 HS256 대비책. 값이 있으면 쓴다
	AllowedOrigins  []string
	Port            string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:     env("DATABASE_URL"),
		SupabaseURL:     strings.TrimRight(env("SUPABASE_URL"), "/"),
		SupabaseAnonKey: env("SUPABASE_ANON_KEY"),
		JWKSURL:         env("SUPABASE_JWKS_URL"),
		JWTSecret:       env("SUPABASE_JWT_SECRET"),
		Port:            env("PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}

	if cfg.DatabaseURL != "" {
		cfg.Driver = "postgres"
		cfg.AuthMode = "supabase"
		if cfg.SupabaseURL == "" || cfg.SupabaseAnonKey == "" {
			return nil, fmt.Errorf("SUPABASE_URL and SUPABASE_ANON_KEY are required")
		}
		if cfg.JWKSURL == "" && cfg.JWTSecret == "" {
			cfg.JWKSURL = cfg.SupabaseURL + "/auth/v1/.well-known/jwks.json"
		}
		return cfg, nil
	}

	// DATABASE_URL이 없으면 SQLite 파일로 1인 로컬 모드를 돈다. Vercel에서는
	// 함수의 일회용 파일시스템에 조용히 쓰게 되므로, 거기서 DATABASE_URL이
	// 없는 것은 언제나 설정 실수다.
	if os.Getenv("VERCEL") != "" {
		return nil, fmt.Errorf("DATABASE_URL is required on Vercel")
	}
	cfg.Driver = "sqlite"
	cfg.AuthMode = "local"
	cfg.SQLitePath = env("SQLITE_PATH")
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = "local-db/flashcard.db"
	}
	return cfg, nil
}
