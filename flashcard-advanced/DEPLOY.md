# 배포 가이드 (전부 무료 티어)

순서대로 진행하면 됩니다. 직접 해야 하는 작업은 ✋ 표시.

## 1. Supabase 프로젝트 ✋

1. https://supabase.com → New project, 리전은 **East US (North Virginia)** 선택
   (Vercel 함수 기본 리전 iad1과 같은 곳이어야 API↔DB가 빠릅니다. 서울 리전을 고르면 안 됩니다.)
2. 프로젝트 대시보드 → **Connect** 버튼에서 연결 문자열 2개 복사:
   - **Transaction pooler (port 6543)** → `DATABASE_URL` (서버용)
   - **Direct connection (port 5432)** 또는 Session pooler → `MIGRATE_DATABASE_URL` (마이그레이션용)
3. Settings → API에서 **Project URL**(`SUPABASE_URL`)과 **anon(public) key**(`SUPABASE_ANON_KEY`) 복사
4. 같은 방법으로 **개발용 프로젝트**를 하나 더 생성 (운영 DB/개발 DB 분리, 이유는 책 36장 "환경 변수와 시크릿" 참고).
   개발용 프로젝트의 값들은 로컬 개발과 Vercel **Preview** 스코프(6단계)에 씁니다.

## 2. DB 마이그레이션

첫 1회는 손으로, 이후에는 GitHub Actions가 자동으로 합니다.

```bash
MIGRATE_DATABASE_URL='<direct 연결 문자열>' go run ./cmd/migrate
```

**운영·개발 프로젝트 각각** 실행합니다 (연결 문자열만 바꿔서 두 번).
Supabase Table Editor에 `profiles, decks, cards, card_srs, study_sessions, review_logs, smart_decks` 테이블이 보이면 성공.
모든 테이블은 RLS enabled + 정책 0개 상태라 anon key로는 PostgREST 접근이 차단됩니다 (Go 서버만 접근 가능).

## 3. OAuth 앱 등록 ✋

> 대시보드 메뉴 이름은 자주 바뀝니다. 책 부록 B("배포 준비: Supabase·Google·GitHub·Vercel 설정")에 화면 캡처와 함께 정리해 두었으니,
> 아래 경로가 화면과 다르면 부록 B를 참고하세요. 여기서는 무엇을 어디에 등록하는지를 기준으로 적습니다.

로그인은 세 관문을 지납니다. 세 곳의 설정이 모두 있어야 로그인이 완성됩니다(원리는 책 32장 "Supabase 인증", 실제 장애 기록은 [notes/2026-07-login-outage.md](./notes/2026-07-login-outage.md)).
① 앱 → Supabase가 프로바이더를 켜 두어야 하고, ② Supabase → OAuth 프로바이더가 Supabase 콜백을 승인해야 하며, ③ OAuth 프로바이더 → 앱이 Supabase의 Redirect URLs에 있어야 합니다.

**Google** — https://console.cloud.google.com

옛 "APIs & Services → OAuth consent screen / Credentials" 메뉴는 이제 **Google 인증 플랫폼(Google Auth Platform)** 아래로 재편됐습니다(왼쪽 메뉴: 개요·브랜딩·대상·클라이언트).

1. Google 인증 플랫폼 → **브랜딩(OAuth consent screen에 해당)**: User Type을 External, 앱 이름 지정
2. Google 인증 플랫폼 → **클라이언트 → 클라이언트 만들기 → 애플리케이션 유형: 웹 애플리케이션**
3. **승인된 리디렉션 URI(Authorized redirect URIs)**: `https://<project-ref>.supabase.co/auth/v1/callback`
   (앱 주소가 아니라 **Supabase 주소**입니다. 앱의 `/auth/callback`은 Google이 아니라 Supabase가 호출하는 두 번째 홉이라 여기 넣지 않습니다.)
4. Client ID/Secret을 Supabase → Authentication → **Sign In / Providers → Google**에 입력하고 Enable
   (저장 반영에 몇 분 걸릴 수 있습니다.)

**GitHub** — https://github.com/settings/developers
1. New OAuth App, **Authorization callback URL**: `https://<project-ref>.supabase.co/auth/v1/callback`
2. Client ID/Secret을 Supabase → **Sign In / Providers → GitHub**에 입력하고 Enable

**개발용 Supabase 프로젝트는 OAuth 클라이언트를 분리하는 편을 권합니다.**
한 클라이언트에 dev 콜백을 추가해 재사용해도 동작하지만(그 경우 Google 클라이언트의 승인된 리디렉션 URI와 GitHub OAuth App의 callback URL에
`https://<dev-project-ref>.supabase.co/auth/v1/callback`을 **추가**), dev와 운영이 같은 client_id/secret을 공유하게 됩니다.
프로젝트별로 클라이언트를 따로 만들면 dev용 값을 개발 Supabase 프로젝트의 Providers에만 넣어 두 환경이 완전히 분리됩니다.

**Redirect URL 등록** — Supabase → Authentication → **URL Configuration**

화면에는 **Site URL** 칸과 **Redirect URLs** 칸이 있습니다("Additional Redirect URLs"는 옛 이름입니다).
Site URL은 "허용 목록에 맞는 게 없을 때의 기본 리다이렉트"이자 폴백입니다. Redirect URLs에 없는 주소로 돌아오면 GoTrue가 **에러 없이 Site URL로 대신 보냅니다**.
그래서 이 값이 개발용 `localhost`로 남아 있으면, 배포에서 로그인한 사용자가 로그인 끝에 `localhost`로 튕깁니다(실제 사례는 notes/2026-07-login-outage.md).

운영 프로젝트:
- Site URL: `https://<앱>.vercel.app` (배포 후 실제 URL로. 커스텀 도메인이 있으면 그 주소로)
- Redirect URLs: `https://<앱>.vercel.app/auth/callback`

개발 프로젝트:
- Site URL: `https://<앱>-git-main-<계정>.vercel.app` 또는 Preview의 고정 도메인(`flashcard-dev.vercel.app` 같은
  대표 별칭). 배포마다 바뀌는 고유 URL이 아니라 고정 주소를 등록해야 합니다.
- Redirect URLs: 위 고정 주소의 `/auth/callback`, 그리고 `http://localhost:8080/auth/callback` (로컬에서 개발 DB에 붙을 때)
- 주의: 기존 항목을 편집할 때 값 중간에 새 URL을 붙여넣어 두 주소가 뭉개지지 않게 합니다. 한 항목에 하나의 완전한 URL만 넣습니다.

## 4. 로컬에서 확인

`.env.dev.example`을 `.env.dev`로 복사해 **개발 프로젝트** 값을 채우고:

```bash
./run_dev.sh          # 개발 DB + GitHub/Google 로그인으로 서버 실행
./migrate_dev.sh      # (필요시) 개발 DB에 마이그레이션 적용
```

`.env.dev`를 읽는 곳은 이 두 스크립트뿐이다. 값을 셸에 상주시키는 도구(direnv 등)는 쓰지 않는다.
`psql`처럼 일회성 명령에 값이 필요하면 그 셸에서 한 번만 읽어 온다:

```bash
set -a; source .env.dev; set +a
psql "$MIGRATE_DATABASE_URL"
```

`run_local.sh`는 `DATABASE_URL`을 지우고(`env -u`) 실행하므로, 그 상태에서도 로컬 모드는 SQLite로 뜬다.

- http://localhost:8080 → Google/GitHub 로그인 왕복 확인
- 덱 만들기 → 카드 추가("사전에서 채우기" 시험) → 학습(일부러 틀려서 재도전 라운드 확인)
- `curl http://localhost:8080/api/healthz` → `{"ok":true}`
- 토큰 없이 `curl http://localhost:8080/api/me` → 401 확인

## 5. GitHub 저장소 ✋

```bash
git remote add origin git@github.com:benelog/flashcard.git
git push -u origin main
git branch release && git push origin release   # 운영 배포용 브랜치 (main = 개발용)
```

**마이그레이션 시크릿 등록** — Settings → Secrets and variables → Actions → New repository secret

| 이름 | 값 |
|---|---|
| `DEV_MIGRATE_DATABASE_URL` | 개발 프로젝트의 **Session pooler / direct (5432)** 문자열 |
| `PROD_MIGRATE_DATABASE_URL` | 운영 프로젝트의 **Session pooler / direct (5432)** 문자열 |

이후 `internal/db/migrations/`가 바뀐 커밋을 푸시하면 `.github/workflows/migrate.yml`이
자동으로 마이그레이션을 적용합니다 — main이면 개발 DB, release면 운영 DB.
포트 6543(transaction pooler)은 advisory lock을 지원하지 않아 여기 쓰면 실패합니다.
GitHub Actions 러너는 IPv4라, Supabase의 IPv6 전용 direct 호스트(`db.<ref>.supabase.co`) 대신
**Session pooler 문자열**을 쓰는 편이 안전합니다.

## 6. Vercel ✋

1. https://vercel.com → Add New Project → flashcard 저장소 import (Root Directory를 `flashcard-advanced`로 지정.
   저장소 루트에는 책 원고와 연습용 코드도 함께 있어 루트 그대로 두면 앱을 찾지 못함.
   Framework Preset은 `vercel.json`의 `"framework": null`이 덮어쓰므로 무엇으로 감지되든 상관없음)
2. **Settings → Environments → Production**을 열고, Branch Tracking의 **Branch is** 값을 `main`에서 `release`로 변경
   (Environments 목록 화면에서 Production·Preview·Development 세 환경의 브랜치와 도메인을 한눈에 확인할 수 있습니다.
   이후 release 푸시/병합 = 운영 배포, main 푸시 = Preview 배포 = 개발 확인용 고유 URL)
3. Environment Variables 등록 — 스코프를 나눠 **Production에는 운영 Supabase 프로젝트 값, Preview에는 개발 프로젝트 값**을 넣습니다 (분리 이유는 책 36장 "환경 변수와 시크릿"):
   | 이름 | 값 |
   |---|---|
   | `DATABASE_URL` | Transaction pooler 문자열 (6543) |
   | `SUPABASE_URL` | `https://<ref>.supabase.co` |
   | `SUPABASE_ANON_KEY` | anon key |
4. 환경 변수는 **다음 배포부터** 적용됩니다. 값을 바꿨다면 main에 푸시하거나 대시보드에서 Redeploy 하세요.
   환경 변수는 이 화면(각 환경 상세)의 **Environment Variables** 구역에서 스코프별로 넣거나, 왼쪽 메뉴 **Environment Variables**에서 한꺼번에 관리합니다.
5. `git push origin release`(또는 main→release 병합)로 운영 배포 → 발급된 URL을 Supabase URL Configuration(3단계)에 반영
6. 개발 환경 분리 확인: main Preview URL에서 로그인·덱 생성 후, 데이터가 **개발** 프로젝트 Table Editor에만 쌓이고 운영 프로젝트에는 없는지 확인

`vercel.json`이 **모든 경로**를 Go 함수(`api/index.go`) 하나로 라우팅합니다. HTML 페이지·정적 파일·API를 이 함수가 전부 서빙하며(정적 파일은 Go 바이너리에 embed), 함수 리전은 iad1로 고정되어 있습니다.

## 7. Android에 설치

배포된 URL을 Android Chrome으로 열고 → 메뉴 → **"홈 화면에 추가"**.
독립 실행형(standalone) PWA로 설치되어 앱처럼 동작합니다. iOS Safari도 "홈 화면에 추가"로 동일하게 사용 가능.

## 문제 해결

- **빌드가 `next build`를 실행하다 "Couldn't find any pages or app directory"로 실패**: 저장소가 Next.js였던 시절 import되어 Framework Preset에 Next.js가 남아 있는 경우. `vercel.json`의 `"framework": null`이 프리셋을 덮어쓰므로, 이 설정이 포함된 커밋을 다시 배포하면 된다.
- **로그인 후 다시 로그인 페이지로, 또는 로그인 끝에 `localhost`로 튕김**: Supabase URL Configuration의 Redirect URLs에 배포 URL의 `/auth/callback`이 없어 GoTrue가 Site URL로 폴백한 경우입니다. Redirect URLs와 Site URL을 함께 확인하세요. 또는 PKCE 쿠키(`fc_pkce`, 300초) 만료입니다(로그인 화면에서 5분 이상 지체하면 실패).
- **`provider is not enabled`(400)로 로그인 실패**: 그 Supabase 프로젝트에서 해당 프로바이더가 꺼져 있습니다. Sign In / Providers에서 Enable하세요. 개발 프로젝트를 새로 만들면 프로바이더 설정이 프로젝트마다 따로라 흔히 빠집니다.
- **Google/GitHub 화면에서 `redirect_uri_mismatch`**: OAuth 클라이언트에 그 Supabase 프로젝트의 콜백(`https://<project-ref>.supabase.co/auth/v1/callback`)이 등록되지 않았습니다. 앱 주소가 아니라 Supabase 주소를 등록해야 합니다.
- **로그인이 실패하고 Vercel Logs에 `net/http: invalid header field value`**: 환경 변수 값에 개행·제어 문자가 섞인 경우입니다(긴 키를 복사할 때 흔합니다). 대시보드에서 값을 한 줄로 다시 입력하세요. 실제 사고 기록은 [notes/2026-07-login-outage.md](./notes/2026-07-login-outage.md) 참고.
- **서버가 500 "jwks unavailable"**: `SUPABASE_URL` 오타(JWKS URL은 여기서 유도). 구형 프로젝트(HS256)라면 대신 `SUPABASE_JWT_SECRET`(Settings → API → JWT Secret)을 설정해도 됩니다.
- **첫 요청이 느림**: 서버리스 콜드스타트 + JWKS 첫 조회. 이후 요청은 빠릅니다.
- **DB 연결 오류 "prepared statement"**: `DATABASE_URL`이 direct(5432)로 되어 있는 경우 transaction pooler(6543)로 교체.
- **오랜만에 Preview를 열었더니 DB 연결 실패**: Supabase 무료 프로젝트는 7일간 미사용 시 자동 정지됩니다.
  대시보드에서 개발 프로젝트를 Restore(unpause)하면 됩니다. 정지된 프로젝트는 무료 2개 한도에 포함되지 않습니다.
- **Preview URL이 Vercel 로그인을 요구(로그인 시도가 `vercel.com/sso-api`로 가로채짐)**: Settings → **Deployment Protection → Vercel Authentication**의 **Require Log In**을 끄세요(Hobby 플랜에서 가능). 이게 켜져 있으면 OAuth 왕복이 앱에 닿기도 전에 막힙니다.
