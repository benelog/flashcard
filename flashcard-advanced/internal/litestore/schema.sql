-- 로컬 단일 사용자 모드를 위한 internal/db/migrations/*.up.sql의 SQLite 포팅본.
-- 멱등 스크립트 하나로 합쳐 두었다(create ... if not exists).
--
-- Ported through: 000004_deck_seq
-- 새 마이그레이션을 여기 옮긴 뒤 위 줄도 함께 고친다. schema_test.go가 이 표식과
-- 마이그레이션 디렉터리를 대조해, 옮기는 것을 잊으면 테스트로 알려 준다.
--
-- 방언 대응: uuid -> text(Go에서 생성), timestamptz -> 고정 폭 UTC 형식의
-- text(timeLayout 참고), jsonb -> text(JSON), text[] -> text(JSON 배열),
-- enum -> text + check. gen_random_uuid()나 now()가 필요한 기본값은 Go가 대신
-- 채운다. deck.seq는 identity 열 대신 insert 시 max(seq)+1로 만든다.
-- Postgres에만 있는 것(RLS, role grant, GIN tags 인덱스)은 뺐다.

create table if not exists profiles (
  id text primary key,
  display_name text,
  settings text not null default '{}',
  created_at text not null
);

create table if not exists decks (
  id text primary key,
  user_id text not null references profiles(id) on delete cascade,
  name text not null,
  description text,
  share_slug text,
  shared_at text,
  seq integer not null unique,
  created_at text not null,
  updated_at text not null
);

create table if not exists cards (
  id text primary key,
  user_id text not null references profiles(id) on delete cascade,
  deck_id text not null references decks(id) on delete cascade,
  text text not null,
  meaning text not null,
  card_type text not null default 'word'
    check (card_type in ('word', 'sentence', 'idiom', 'concept')),
  tags text not null default '[]',
  phonetic text,
  example text,
  notes text,
  created_at text not null,
  updated_at text not null
);

create table if not exists card_srs (
  card_id text primary key references cards(id) on delete cascade,
  user_id text not null references profiles(id) on delete cascade,
  ease_factor real not null default 2.5,
  interval_days real not null default 0,
  repetitions integer not null default 0,
  lapses integer not null default 0,
  due_at text not null,
  last_reviewed_at text,
  correct_count integer not null default 0,
  incorrect_count integer not null default 0
);

create table if not exists study_sessions (
  id text primary key,
  user_id text not null references profiles(id) on delete cascade,
  mode text not null check (mode in ('deck', 'due', 'smart')),
  direction text not null default 'text_to_meaning'
    check (direction in ('text_to_meaning', 'meaning_to_text')),
  deck_id text references decks(id) on delete set null,
  smart_rule text,
  total_cards integer not null default 0,
  started_at text not null,
  ended_at text,
  completed integer not null default 0
);

create table if not exists review_logs (
  id integer primary key autoincrement,
  user_id text not null references profiles(id) on delete cascade,
  card_id text not null references cards(id) on delete cascade,
  session_id text references study_sessions(id) on delete set null,
  result integer not null,
  is_retry integer not null default 0,
  reviewed_at text not null
);

create table if not exists smart_decks (
  id text primary key,
  user_id text not null references profiles(id) on delete cascade,
  name text not null,
  rule text not null,
  created_at text not null
);

create view if not exists cards_with_stats as
select
  c.id, c.user_id, c.deck_id, c.text, c.meaning, c.card_type,
  c.tags, c.phonetic, c.example, c.notes, c.created_at,
  s.ease_factor, s.interval_days, s.repetitions, s.lapses, s.due_at,
  s.last_reviewed_at, s.correct_count, s.incorrect_count,
  (s.correct_count + s.incorrect_count) as attempts,
  case when s.correct_count + s.incorrect_count = 0 then 0.0
       else cast(s.incorrect_count as real) / (s.correct_count + s.incorrect_count)
  end as error_rate
from cards c
join card_srs s on s.card_id = c.id;

create index if not exists decks_user_idx on decks (user_id);
create unique index if not exists decks_share_slug_idx on decks (share_slug)
  where share_slug is not null;
create index if not exists cards_user_idx on cards (user_id);
create index if not exists cards_deck_idx on cards (deck_id);
create index if not exists card_srs_user_due_idx on card_srs (user_id, due_at);
create index if not exists review_logs_user_time_idx on review_logs (user_id, reviewed_at);
create index if not exists review_logs_card_idx on review_logs (card_id);
create index if not exists study_sessions_user_idx on study_sessions (user_id, started_at);
create index if not exists smart_decks_user_idx on smart_decks (user_id);
