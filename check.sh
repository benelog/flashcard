#!/usr/bin/env bash
# 커밋·push 전 종합 검증. 사람도 에이전트도 같은 명령 하나로 돌린다.
# 성공하면 한 줄만 찍고, 실패하면 첫 실패에서 멈춘다.
set -euo pipefail
cd "$(dirname "$0")"

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "gofmt 위반:" >&2
  echo "$unformatted" >&2
  exit 1
fi

# 저장소에 Go 모듈이 셋이다. 모듈마다 디렉터리에 들어가 돌린다.
for m in flashcard-advanced flashcard-basic flashcard-cli; do
  (cd "$m" && go build ./... && go vet ./... && go test ./... >/dev/null)
done
(cd language-basic/go && go build ./... && go vet ./...)

# Vercel은 api/index.go를 모듈 밖에서 단독 컴파일한다.
# internal/ 패키지를 import하면 로컬 빌드는 통과해도 배포가 실패하므로 여기서 미리 잡는다.
(cd flashcard-advanced && go build -o /dev/null api/index.go)

echo "PASS: gofmt, build, vet, test, vercel"
