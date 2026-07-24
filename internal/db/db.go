package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool     *pgxpool.Pool
	poolOnce sync.Once
	poolErr  error
)

// Pool은 프로세스 전역 pgx 풀을 돌려준다. Vercel에서는 따뜻한 함수 인스턴스가
// 호출 사이에 풀을 재사용하므로 작게 유지한다. Supabase의 풀링 포트(Supavisor
// 트랜잭션 모드)는 prepared statement도 허용하지 않아 simple protocol을 쓴다.
func Pool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolOnce.Do(func() {
		cfg, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			poolErr = fmt.Errorf("parse DATABASE_URL: %w", err)
			return
		}
		cfg.MaxConns = 4
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		pool, poolErr = pgxpool.NewWithConfig(ctx, cfg)
	})
	return pool, poolErr
}
