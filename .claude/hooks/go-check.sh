#!/usr/bin/env bash
# PostToolUse 훅: 수정된 Go 파일에 gofmt를 적용하고 go vet을 돌린다.
# 성공하면 조용히 끝난다(LLM 토큰 0). vet 오류는 exit 2로 Claude에게 되돌린다.
set -u

f=$(jq -r '.tool_input.file_path // .tool_response.filePath // empty')
case "$f" in
  *.go) ;;
  *) exit 0 ;;
esac
[ -f "$f" ] || exit 0

gofmt -w "$f" 2>/dev/null

# 저장소에 Go 모듈이 셋이다(flashcard-advanced, flashcard-basic, flashcard-cli).
# 파일이 속한 디렉터리에서 vet을 돌리면 go가 알아서 그 모듈의 go.mod를 찾는다.
dir=$(dirname "$f")
if ! out=$(cd "$dir" && go vet . 2>&1); then
  {
    echo "go vet failed for $dir:"
    echo "$out"
  } >&2
  exit 2
fi
exit 0
