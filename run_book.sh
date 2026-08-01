#!/usr/bin/env bash
# 이북 뷰어(book/) 로컬 실행: 원고를 감시하며 VitePress 개발 서버를 띄운다.
# 처음 한 번은 book-template(빌드 엔진)과 book의 npm 의존성을 자동으로 설치한다.
#
#   ./run_book.sh           원고 감시 + 개발 서버 (http://localhost:5173/flashcard/)
#   ./run_book.sh build     전체 빌드 (깨진 링크·인용 검증 포함)
#   ./run_book.sh preview   빌드 결과를 그대로 미리보기 (필요하면 먼저 빌드한다)
set -euo pipefail
cd "$(dirname "$0")"

mode="${1:-dev}"
case "$mode" in
dev | build | preview) ;;
*)
	echo "사용법: $0 [dev|build|preview]" >&2
	exit 1
	;;
esac

command -v npm >/dev/null || {
	echo "npm이 없다. Node.js 20 이상을 설치한다." >&2
	exit 1
}

# book은 file:../book-template 를 의존한다. 엔진 쪽 의존성이 먼저 깔려 있어야 book CLI가 돈다.
[ -d book-template/node_modules ] || (cd book-template && npm install)
[ -d book/node_modules ] || (cd book && npm install)

cd book

# preview는 빌드 결과(.vitepress/dist)를 서빙할 뿐이라, 없으면 먼저 만들어 준다.
if [ "$mode" = preview ] && [ ! -d .vitepress/dist ]; then
	npm run build
fi

exec npm run "$mode"
