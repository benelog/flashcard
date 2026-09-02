# 원고 모호 표현 구체화 진도표

2026-09-02 점검에서 나온 지적사항(파일별 약 230건)을 `book/` 원고에 반영한다.
점검 기준과 전체 목록은 이 파일의 "남은 항목" 절과 커밋 이력에 남긴다.

## 통일 값 (코드 실측, 2026-09-02)

| 항목 | 원고에 쓰는 값 | 근거 |
|---|---|---|
| `app.js` 줄 수 | 160줄 남짓 | 162줄 |
| `app.css` 줄 수 | 1,250줄 남짓 | 1,254줄 |
| `sw.js` 줄 수 | 76줄 | 76줄 |
| `srs.go` 줄 수 | 59줄 | 59줄(주석 포함) |
| `Store` 메서드 수 | 35개(서른다섯) | `internal/model/store.go` |
| `pkg/app` 테스트 | 44개(서브테스트까지 79번) | `go test -v` |
| 화면 템플릿 | 13장 | `templates/pages/` |
| `collect` 호출 | pgstore 11곳, 종류 9 | `grep collect(` |
| 핑 주기 | 하루 두 번 | free-tier 기준 |
| 함수 실행 상한 | 300초 | free-tier |
| Supabase 일시정지 | 7일 | free-tier |
| 리전 간 왕복 | 180ms, 요청당 3~4회 | supabase-db |

## 진행 상태

- [x] preface.adoc
- [x] part1/requirements.adoc
- [x] part1/tech-choices.adoc
- [x] part1/claude-code.adoc
- [x] part1/instructing.adoc
- [x] part1/first-run.adoc
- [x] part2/html-hello.adoc
- [x] part2/css-hello.adoc
- [x] part2/go-hello.adoc
- [x] part2/go-types.adoc
- [x] part2/database-basics.adoc
- [x] part2/go-sqlite.adoc
- [x] part2/gin-hello.adoc
- [x] part2/template-hello.adoc
- [x] part3/design-principles.adoc
- [x] part3/database-design.adoc
- [x] part3/domain-model.adoc
- [x] part3/code-map.adoc
- [x] part3/core-logic.adoc
- [x] part3/unit-tests.adoc
- [x] part3/gin.adoc
- [x] part3/app-assembly.adoc
- [x] part3/html-css.adoc
- [x] part3/html-template.adoc
- [x] part3/htmx.adoc
- [x] part3/app-tests.adoc
- [x] part3/local-dev.adoc
- [x] part4/git.adoc
- [x] part4/quality-gates.adoc
- [x] part4/github-actions.adoc
- [x] part4/vercel.adoc
- [x] part4/vercel-config.adoc
- [x] part4/supabase-auth.adoc
- [x] part4/token-verification.adoc
- [x] part4/supabase-db.adoc
- [x] part4/migrations.adoc
- [x] part5/env-secrets.adoc
- [x] part5/pwa.adoc
- [x] part5/caching.adoc
- [x] part5/free-tier.adoc
- [x] part5/whats-next.adoc
- [x] appendix/alternatives.adoc
- [x] appendix/deploy.adoc
- [x] `cd book && npm run build` 통과 (2026-09-02, exit 0)

## 결과 요약

- 43개 원고 파일 전부 반영. 44개 파일 변경(원고 43 + `internal/web/cookies.go` 주석 1), 484줄 추가·379줄 삭제.
- 저장소에서 셀 수 있는 값은 전부 실측값으로 바꿨다(줄 수, 개수, 파일 이름, 함수 이름, 커밋 수, 빌드·테스트·훅 시간).
- 외부 사실은 공식 문서를 확인해 각주(`footnote:`)로 출처를 달았다(약 35건). 확인이 안 된 것은 수치 없이 기제만 적었다.
- 사실 정정(원고가 코드·문서와 어긋났던 것): `app.js` 100→162줄(8개 장), `Store` 34→35, 테스트 41→44, `app.css`·`sw.js`·`srs.go` 줄 수, `collect` 반복 8→11곳, `default` 열 13/39, 훅이 테스트를 돌린다는 서술, 서비스 워커가 API를 부른다는 서술, 상단 바 `.row` 서술, `RenameDeck`·`srs_test.go`·`stubStore` 서술, 최소 앱을 "만든다"는 문장 6곳.
- 절 제목 하나 변경: `part3/core-logic.adoc` "여덟 곳의 반복을…" → "열 곳 넘는 반복을 하나로 모은 자리"(다른 원고에서 참조 없음).
- 커밋은 하지 않았다.

## 저자 확인 (2026-09-02 완료)

아래 판단은 저자가 확인해 유지하기로 했다.

- Supabase 리프레시 토큰: "기본 유효 기간 30일" → "이 앱이 정한 쿠키 수명 30일"(supabase-auth 461·587, `cookies.go:34`).
- Vercel 함수 리전: "무료 티어라 서울 불가" → "기본값 iad1을 고정했으니 DB를 곁에 둔다"(supabase-db 341·344).
- Supabase 일시정지 복원 기한: 90일 → 1년(free-tier 151·175·200·277).
- PWA 설치 조건: 서비스 워커는 필수 조건에서 빠졌고 캐시를 위해 둔다(pwa 24·43·56·191). `short_name` 길이는 수치 없이.
- claude-code 354·361: "실제로 겪었다" 선언을 빼고 기제만 서술.
- app-tests 표 "1초 안쪽", html-css 399 "가장 자주 어기는" 삭제: 유지.
- 배포 환경 실측이 필요한 다섯 곳(Vercel 콜드스타트, 마이그레이션 잡과 빌드의 시차, Supabase 복원 시간, Node 기동, GraalVM 단축 폭)은 수치 없이 기제만 적은 현재 서술 유지.

## 출간 전 점검

- 시점이 박힌 수치를 다시 잰다. "이 글을 쓸 때(2026년 9월)"로 표기한 곳: `pkg/app` 테스트 44개·서브테스트 79번(app-tests, unit-tests), 커버리지 표(unit-tests 398~432), 커밋 212개 중 204개(git 195), Gin 스타 89,163(tech-choices 344), CI 1분 30초(github-actions 123·239, quality-gates 359).
- 코드가 자라면 어긋나는 값: `app.js` 160줄 남짓, `app.css` 1,250줄 남짓, `sw.js` 76줄, `srs.go` 59줄, `Store` 메서드 35개, 화면 템플릿 13장, `collect` 호출 11·12곳, 환경 변수 8개, hx-post 8곳, 아이콘 31개.
- 새로 단 각주 URL(약 35건)의 링크 생존을 확인한다. `grep -rho 'https://[^[ ]*' book/preface.adoc book/part*/*.adoc book/appendix/*.adoc | sort -u`로 뽑아 HEAD 요청을 보내면 된다.
