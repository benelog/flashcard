#!/usr/bin/env bash
# 운영 배포: main을 release 브랜치에 병합하고 푸시한다.
# release에는 직접 커밋하지 않는다. 이 스크립트가 유일한 운영 배포 경로다.
set -euo pipefail
cd "$(dirname "$0")"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "작업 트리에 커밋되지 않은 변경이 있습니다. 커밋하거나 stash 후 다시 실행하세요." >&2
  exit 1
fi

current_branch="$(git rev-parse --abbrev-ref HEAD)"

echo "==> 검증: go build / vet / test"
go build ./...
go vet ./...
go test ./...

echo "==> 최신 원격 상태 가져오기"
git fetch origin

echo "==> main 푸시 (개발 환경 배포)"
git checkout main
git merge --ff-only origin/main
git push origin main

echo "==> release에 main 병합"
git checkout release
git merge --ff-only origin/release
git merge --no-edit main

echo "==> release 푸시 (운영 배포)"
git push origin release

git checkout "$current_branch"
echo "==> 완료: main → release 병합·푸시가 끝났습니다."
