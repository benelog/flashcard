create type card_type as enum ('word', 'sentence', 'idiom');
create type session_mode as enum ('deck', 'due', 'smart');

create table profiles (
  id uuid primary key,
  display_name text,
  settings jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create table decks (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  name text not null,
  description text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table cards (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  deck_id uuid not null references decks(id) on delete cascade,
  front_text text not null,
  back_text text not null,
  card_type card_type not null default 'word',
  tags text[] not null default '{}',
  phonetic text,
  example text,
  notes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table card_srs (
  card_id uuid primary key references cards(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  ease_factor real not null default 2.5,
  interval_days real not null default 0,
  repetitions int not null default 0,
  lapses int not null default 0,
  due_at timestamptz not null default now(),
  last_reviewed_at timestamptz,
  correct_count int not null default 0,
  incorrect_count int not null default 0
);

create table study_sessions (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  mode session_mode not null,
  deck_id uuid references decks(id) on delete set null,
  smart_rule jsonb,
  total_cards int not null default 0,
  started_at timestamptz not null default now(),
  ended_at timestamptz,
  completed boolean not null default false
);

create table review_logs (
  id bigint generated always as identity primary key,
  user_id uuid not null references profiles(id) on delete cascade,
  card_id uuid not null references cards(id) on delete cascade,
  session_id uuid references study_sessions(id) on delete set null,
  result boolean not null,
  is_retry boolean not null default false,
  reviewed_at timestamptz not null default now()
);

create table smart_decks (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  name text not null,
  rule jsonb not null,
  created_at timestamptz not null default now()
);

create view cards_with_stats with (security_invoker = true) as
select
  c.id, c.user_id, c.deck_id, c.front_text, c.back_text, c.card_type,
  c.tags, c.phonetic, c.example, c.notes, c.created_at,
  s.ease_factor, s.interval_days, s.repetitions, s.lapses, s.due_at,
  s.last_reviewed_at, s.correct_count, s.incorrect_count,
  (s.correct_count + s.incorrect_count) as attempts,
  case when s.correct_count + s.incorrect_count = 0 then 0.0
       else s.incorrect_count::float / (s.correct_count + s.incorrect_count)
  end as error_rate
from cards c
join card_srs s on s.card_id = c.id;

create index decks_user_idx on decks (user_id);
create index cards_user_idx on cards (user_id);
create index cards_deck_idx on cards (deck_id);
create index cards_tags_gin on cards using gin (tags);
create index card_srs_user_due_idx on card_srs (user_id, due_at);
create index review_logs_user_time_idx on review_logs (user_id, reviewed_at);
create index review_logs_card_idx on review_logs (card_id);
create index study_sessions_user_idx on study_sessions (user_id, started_at);
create index smart_decks_user_idx on smart_decks (user_id);

-- The Go API is the only client of these tables. Enabling RLS with zero
-- policies blocks Supabase PostgREST access via the anon/authenticated roles,
-- while the table owner (the connection Go uses) bypasses RLS.
alter table profiles enable row level security;
alter table decks enable row level security;
alter table cards enable row level security;
alter table card_srs enable row level security;
alter table study_sessions enable row level security;
alter table review_logs enable row level security;
alter table smart_decks enable row level security;

-- Belt and braces on Supabase: drop the default grants PostgREST roles get.
-- Wrapped so the migration also runs on plain Postgres without these roles.
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on all tables in schema public from anon;
  end if;
  if exists (select 1 from pg_roles where rolname = 'authenticated') then
    revoke all on all tables in schema public from authenticated;
  end if;
end $$;
