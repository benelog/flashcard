#!/usr/bin/env bash
# local에서 dev 환경 DB에 스키마 변경(internal/db/migrations/*.up.sql)을 적용한다.
# main 에 푸시하면 GitHub Actions(.github/workflows/migrate.yml)가 같은 일을 자동으로
# 하지만, 푸시 전에 개발 DB로 먼저 확인하고 싶을 때 이 스크립트를 쓴다.
#
# 운영 DB에는 이 스크립트를 쓰지 않는다 — 운영 반영은 release 브랜치 푸시로만 한다.
set -euo pipefail
cd "$(dirname "$0")"

env_file="${ENV_FILE:-.env.dev}"
if [[ ! -f "$env_file" ]]; then
  echo "$env_file 이 없습니다. .env.dev.example 를 복사해 개발용 Supabase 값을 채우세요." >&2
  exit 1
fi

set -a
# shellcheck source=/dev/null
source "$env_file"
set +a

# 마이그레이션은 advisory lock 을 잡으므로 transaction pooler(6543)가 아니라
# Session pooler / direct connection(5432)이어야 한다.
: "${MIGRATE_DATABASE_URL:?$env_file 에 MIGRATE_DATABASE_URL(5432)이 필요합니다}"
export MIGRATE_DATABASE_URL

echo "==> dev DB 마이그레이션 적용"
exec go run ./cmd/migrate
