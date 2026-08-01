#!/usr/bin/env bash
# dev 환경 실행: 로컬 서버(:8080)를 개발용 Supabase 프로젝트에 붙인다.
# 개발 DB(PostgreSQL) + GitHub/Google 로그인까지 로컬에서 그대로 시험할 수 있다.
# 값은 .env.dev 에서 읽는다 (.env.dev.example 참고, .env.dev 는 커밋되지 않는다).
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

: "${DATABASE_URL:?$env_file 에 DATABASE_URL(Transaction pooler, 6543)이 필요합니다}"
: "${SUPABASE_URL:?$env_file 에 SUPABASE_URL 이 필요합니다}"
: "${SUPABASE_ANON_KEY:?$env_file 에 SUPABASE_ANON_KEY 가 필요합니다}"

echo "==> dev 환경으로 실행 ($SUPABASE_URL) → http://localhost:8080"
exec go run ./cmd/server
