# 대안 플랫폼 무료 티어 조사

개발 환경을 Vercel + Supabase 밖의 플랫폼에 무료(카드 등록 없이)로 구성할 수 있는지 조사한 기록.
조사일: 2026-07-11. 무료 티어 정책은 자주 바뀌므로 인용 전에 재확인할 것.

**결론**: 세 플랫폼 모두 부적합. 개발 환경은 기존 Vercel 프로젝트의 Preview 스코프 환경 변수
+ Supabase 무료 프로젝트 추가(무료 한도 2개)로 구성한다. 절차는 [DEPLOY.md](DEPLOY.md) 참고.

## Fly.io — 카드 등록 필수

- 2024년에 무료 티어(Hobby 플랜의 shared-cpu VM 3대 등) 폐지. 2024-10-07 이후 신규 조직은 종량제 전용.
- 신규 가입 체험은 **VM 2시간 또는 7일 중 먼저 도달하는 쪽**까지. 이후 카드 등록 없이는 사용 불가.
- 소형 앱 기준 실질 최저 비용 월 $5 안팎.
- 판정: "카드 없이 무료" 조건 탈락.
- 출처: [Fly.io Free Trial](https://fly.io/docs/about/free-trial/), [Fly.io Pricing](https://fly.io/docs/about/pricing/)

## Koyeb — 신규 가입은 무료 티어 진입 불가

- 2026-02-17 Mistral AI에 인수. 신규 사용자는 무료 Starter 티어에 가입할 수 없고,
  진입 플랜이 Pro **월 $29 + 컴퓨트 비용**.
- (기존 무료 티어 기준) 무료 인스턴스는 조직당 1개, 512MB RAM / 0.1 vCPU / 2GB SSD,
  1시간 무트래픽 시 scale-to-zero. 무료 Postgres는 1GB, **월 컴퓨트 5시간** 제한.
- 판정: 신규 가입 기준 무료 구성 자체가 불가.
- 출처: [Koyeb Pricing FAQ](https://www.koyeb.com/docs/faqs/pricing), [Koyeb Pricing](https://www.koyeb.com/pricing), [srvrlss.io Koyeb 정리](https://www.srvrlss.io/provider/koyeb/)

## Render — 카드는 불필요하지만 무료 DB가 30일 만료

- 카드 등록 없이 웹 서비스 + PostgreSQL + 정적 사이트 배포 가능 (세 플랫폼 중 유일).
- 그러나 **무료 PostgreSQL은 생성 30일 후 만료** (2024-05-20에 90일 → 30일로 단축).
  만료 후 14일 유예 안에 유료 전환하지 않으면 데이터까지 삭제.
- 무료 웹 서비스는 15분 미사용 시 잠들어 콜드 스타트가 김. 무료 DB 저장 용량 1GB.
- 판정: 상시 유지해야 하는 개발 DB로는 30일 만료가 치명적. DB를 Supabase에 두고
  웹 서비스만 Render에 올리는 변형은 가능하지만, 운영(Vercel)과 배포 방식이 달라져
  환경 불일치가 생기므로 이점이 없음.
- 출처: [Render Changelog — 무료 PostgreSQL 30일 만료](https://render.com/changelog/free-postgresql-instances-now-expire-after-30-days-previously-90), [Render 무료 티어 소개](https://render.com/articles/platforms-with-a-real-free-tier-for-developers-in-2026)

## 채택안과 비교

| 항목 | Vercel Preview + Supabase 2번째 프로젝트 | Fly.io | Koyeb | Render |
|---|---|---|---|---|
| 카드 등록 | 불필요 | **필수** | 사실상 필수(Pro) | 불필요 |
| DB 유지 기간 | 무기한 (7일 미사용 시 일시 정지, 복구 가능) | — | 월 5시간 | **30일 만료** |
| 운영 환경과 일치 | 동일 스택 | 다름 | 다름 | 다름 |
| 추가 플랫폼 | 없음 | +1 | +1 | +1 |

Supabase 무료 프로젝트의 제약: 활성 2개까지, 7일 미사용 시 자동 정지(대시보드에서 수동 복구,
정지 중에는 2개 한도에 미포함), DB 500MB / 파일 1GB / 대역폭 5GB.
출처: [Supabase Pricing](https://supabase.com/pricing), [Supabase Billing FAQ](https://supabase.com/docs/guides/platform/billing-faq)
