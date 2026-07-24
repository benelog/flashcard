# 로그인 실패 사후 기록

> 이 문서는 과거 로그인 장애의 기록이다. 진행 중인 작업 목록이 아니다.
> 두 건을 담았다. 2026-07-11 배포 정상화 직후의 세 원인, 2026-07-15 개발 프로젝트를 새로 배선하며 겪은 세 관문이다. 둘 다 종결됐다.
> 조사 중에 세운 가설과 폐기된 시도는 걷어냈고, 다시 겪지 않기 위해 남길 값이 있는 것만 남겼다.

## 2026-07-11: 배포 정상화 직후 세 원인

2026-07-11, 배포 정상화(환경 변수 등록, `framework: null`) 직후 개발·운영 양쪽에서 로그인이 실패했다.
원인은 서로 무관한 세 가지였고, 하나씩 분리해 잡았다.
조치가 모두 적용됐고, 2026-07-14에 dev·production 양쪽의 배선을 다시 실측해 확인했다.

### 원인 1: 환경 변수 값에 섞인 개행

운영 로그인 실패의 근본 원인이다.

Vercel에 등록된 `SUPABASE_ANON_KEY` 값 **중간에 개행 문자**가 들어 있었다(긴 키를 터미널에서 복사할 때 줄바꿈이 섞인 형태로 추정).
Go의 HTTP 클라이언트는 헤더 값에 제어 문자가 있으면 요청을 보내지 않고 거부하므로, 토큰 교환 요청이 GoTrue에 **도달조차 하지 못했다**.
진단 패치 배포 후 Vercel Logs에 찍힌 `net/http: invalid header field value for "Apikey"`가 결정적 증거였고, GoTrue의 flow state가 소비되지 않고 살아 있던 정황과도 일치했다.

조치는 두 단계였다.
값 자체는 저자가 대시보드에서 한 줄로 재입력해 근본 해결했다.
코드 쪽에는 `internal/config`가 모든 환경 변수를 `strings.TrimSpace`로 읽는 방어와 회귀 테스트(`TestLoadTrimsWhitespace`)를 남겼다.
값 **중간**의 공백까지 지우던 임시 조치(`envKey`)는 값이 정상이 된 뒤 제거했다.
설정 값을 코드가 조용히 변형하기 시작하면, 잘못된 값이 잘못된 채로 동작해 다음 사고를 부르기 때문이다.

### 원인 2: Preview 배포가 Vercel 인증에 막혀 있었다

개발 환경에는 로그인 시도가 앱에 닿지도 못했다.
Vercel이 Preview 배포에 기본으로 켜 두는 Deployment Protection(Vercel Authentication)이 모든 요청을 가로채, `/auth/login/google` 요청이 앱이 아니라 `https://vercel.com/sso-api?...`로 리다이렉트되고 있었다.
OAuth 콜백 왕복도 같은 벽에 막힌다.

조치: Settings → Deployment Protection → Vercel Authentication을 **Disabled**로(Hobby 플랜에서 가능).

### 원인 3: 브랜치와 환경이 뒤바뀌어 있었다

Vercel의 Production Branch가 `main`으로 설정되어 있어서, **main에 푸시하면 곧장 운영 도메인에 운영 Supabase로 배포**됐다.
의도한 정책(14·16장, DEPLOY.md)은 그 반대다.
개발과 운영의 분리 자체가 성립하지 않던 상태였다.

조치: Settings → Environments → Production → Branch Tracking을 `release`로 변경.

### 현재 구성 (2026-07-14 실측)

| 환경 | URL | Supabase 프로젝트 |
|---|---|---|
| dev | https://flashcard-dev.vercel.app | `aueafzjlmqdtcfrkctzx` (개발) |
| production | https://flashcard.benelog.net | `kncwqneczvkugkflqwpe` (운영) |

각 배포의 `/auth/login/google` 리다이렉트가 서로 다른 프로젝트를 가리키는 것으로 확인했다.
Vercel의 Preview 스코프에는 개발 프로젝트 값이, Production 스코프에는 운영 프로젝트 값이 들어 있다는 뜻이다.
스키마 적용은 `.github/workflows/migrate.yml`이 브랜치를 보고 대신 한다(main → 개발 DB, release → 운영 DB).

### 조사 중 발견해 고친 진단 공백 (적용 완료)

원인 추적을 막았던 두 곳이다. 원인 확정과 별개로 고쳐 두었다.

- `internal/web/authpages.go`: 콜백이 GoTrue의 `error`·`error_description`을 검사하지 않고 버려서, 프로바이더 쪽 실패가 "code 없음"과 구분되지 않았다. 지금은 로그로 남기고 사용자 메시지도 구분한다.
- `internal/web/gotrue.go`: 토큰 교환 실패 시 응답 본문을 버리고 상태 코드만 남겨서(`status 400`) 로그만 봐서는 원인을 알 수 없었다. 지금은 본문을 에러에 담는다.

## 2026-07-15: 개발 프로젝트 재배선, 세 관문

개발용 Supabase 프로젝트(`aueafzjlmqdtcfrkctzx`)에 Google 로그인을 처음 붙이는 과정에서, 로그인이 세 단계에 걸쳐 차례로 막혔다.
하나를 풀면 다음이 드러나는 식이라, 세 곳을 모두 채워야 로그인이 완성됐다.
원인은 모두 대시보드 설정 누락이었고 저장소 코드는 바뀌지 않았다.

로그인 왕복은 세 관문을 지난다. 관문마다 막혔을 때의 증상이 다르고, 그 증상이 어느 설정이 빠졌는지를 가리킨다.

**첫째, 앱이 Supabase GoTrue의 `authorize`로 보낸다.**
개발 프로젝트에 Google 프로바이더가 꺼져 있으면 여기서 막힌다.
GoTrue가 리다이렉트 없이 `400 {"error_code":"validation_failed","msg":"Unsupported provider: provider is not enabled"}`을 돌려준다.
조치: Supabase → Authentication → Sign In / Providers → Google에 Client ID/Secret을 넣고 Enable.

**둘째, GoTrue가 Google 동의 화면으로 보낸다.**
Google OAuth 클라이언트에 Supabase 콜백이 등록되어 있지 않으면 여기서 막힌다.
Google이 `redirect_uri_mismatch`로 거부하며, 문제의 URI(`https://<project-ref>.supabase.co/auth/v1/callback`)를 에러에 담아 알려 준다.
여기 등록하는 값은 앱 주소(`flashcard-dev.vercel.app`)가 아니라 Supabase 주소다. 앱의 `/auth/callback`은 Google이 아니라 GoTrue가 호출하는 두 번째 홉이기 때문이다.
조치: Google Cloud Console → Google 인증 플랫폼 → 클라이언트 → 승인된 리디렉션 URI에 Supabase 콜백 추가.

**셋째, Google이 다시 GoTrue를 거쳐 앱의 `/auth/callback`으로 돌려보낸다.**
이때 앱이 넘긴 `redirect_to`(`https://flashcard-dev.vercel.app/auth/callback`)가 Supabase의 Redirect URLs 목록에도 없고 Site URL 호스트와도 다르면, GoTrue가 조용히 Site URL로 대신 보낸다.
개발 프로젝트의 Site URL이 로컬 작업용 `http://localhost:8080`으로 되어 있어서, 로그인 마지막에 `localhost:8080`으로 튕겼다.
에러가 아니라 조용한 대체라 원인을 짐작하기 어렵다.
조치: Supabase → Authentication → URL Configuration에서 Site URL을 `https://flashcard-dev.vercel.app`으로 바꾸고, Redirect URLs에 `https://flashcard-dev.vercel.app/auth/callback`과 `http://localhost:8080/auth/callback`을 등록.

### 지금 로그인이 되는 진짜 이유 (남은 정리 작업)

셋째 관문은 Redirect URLs 목록이 아니라 Site URL 덕에 통과하고 있다.
현재 개발 프로젝트의 Redirect URLs에는 붙여넣기 중 뭉개진 항목이 하나 들어 있다.

```
https://flashcard-dev.vercel.app/authhttps://flashcard-dev.vercel.app/auth/callback/callback
```

기존 값(`…/auth/callback`) 중간에 새 값을 끼워 넣어 두 URL이 붙었다.
그래서 깨끗한 `https://flashcard-dev.vercel.app/auth/callback`은 목록에 없다.

그런데도 로그인이 되는 것은 GoTrue의 `IsRedirectURLValid`가 허용 목록에 없어도 **`redirect_to`의 호스트·스킴·포트가 Site URL과 같으면 통과**시키기 때문이다(auth 저장소 `internal/utilities/request.go`).
Site URL을 `https://flashcard-dev.vercel.app`으로 바꾼 것이 실제로 셋째 관문을 연 조치다.

이 상태는 Site URL을 다른 값으로 바꾸는 순간 깨진다.
뭉개진 항목을 지우고 `https://flashcard-dev.vercel.app/auth/callback`을 다시 한 줄로 등록해 두는 것이 안전하다.

## 다시 겪지 않기 위한 점검 순서

로그인이 실패하면 이 순서로 좁힌다. 앞의 세 항목은 세 관문에 하나씩 대응한다.

1. **콜백 URL을 본다.** 실패 직후 브라우저 방문 기록에서 `/auth/callback`을 찾는다. `?error=...`가 붙어 있으면 GoTrue나 OAuth 프로바이더 쪽 문제이고(`error_description`이 원인 문장이다), `?code=...`만 있으면 앱 쪽 문제다.
2. **authorize 응답이 `provider is not enabled`(400)인지 본다(관문 1).** 그 프로바이더가 그 Supabase 프로젝트에서 꺼져 있다. Sign In / Providers에서 Enable한다. 개발 프로젝트를 새로 만들면 프로바이더 설정이 프로젝트마다 따로라 흔히 빠진다.
3. **Google·GitHub 화면이 `redirect_uri_mismatch`인지 본다(관문 2).** OAuth 클라이언트에 그 Supabase 프로젝트의 콜백(`https://<project-ref>.supabase.co/auth/v1/callback`)이 없다. 앱 주소가 아니라 Supabase 주소를 등록한다.
4. **로그인 끝에 엉뚱한 주소(예: `localhost:8080`)로 가는지 본다(관문 3).** Redirect URLs에 그 배포의 `/auth/callback`이 없어서 Site URL로 조용히 폴백된 것이다. Redirect URLs 목록과 Site URL을 함께 확인한다. 배포 주소가 바뀌면 여기도 함께 바꾼다.
5. **Vercel Logs를 본다.** `net/http: invalid header field value`가 보이면 환경 변수 값에 개행이나 제어 문자가 섞인 것이다. 값을 다시 한 줄로 입력한다.
6. **PKCE 쿠키 만료를 의심한다.** `fc_pkce`는 Max-Age 300초다. 구글 계정 선택 화면에서 5분 이상 지체하면 콜백 시점에 verifier가 없어 실패한다. 간헐적 실패의 흔한 원인이다.
