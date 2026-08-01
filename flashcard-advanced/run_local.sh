#!/usr/bin/env bash
# 로컬 모드 실행: Go 서버 하나(:8080)가 HTML 페이지와 API를 모두 서빙한다.
# 환경 변수 불필요, SQLite 파일(local-db/flashcard.db) 사용, 로그인 없음.
#
# DATABASE_URL 을 지우고 실행한다. 어떤 이유로든 개발 DB 값이 셸에 올라와 있어도
# 로컬 모드는 언제나 SQLite 로 뜨게 하기 위함이다. 개발 DB에 붙으려면 ./run_dev.sh 를 쓴다.
set -euo pipefail
cd "$(dirname "$0")"

exec env -u DATABASE_URL go run ./cmd/server
