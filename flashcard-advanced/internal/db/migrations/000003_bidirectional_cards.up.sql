-- Cards are bidirectional: rename front/back to text (the expression/term
-- being learned) and meaning (its definition), add the 'concept' card type,
-- and record the study direction per session.

alter type card_type add value if not exists 'concept';

create type study_direction as enum ('text_to_meaning', 'meaning_to_text');

alter table study_sessions
  add column direction study_direction not null default 'text_to_meaning';

drop view cards_with_stats;

alter table cards rename column front_text to text;
alter table cards rename column back_text to meaning;

create view cards_with_stats with (security_invoker = true) as
select
  c.id, c.user_id, c.deck_id, c.text, c.meaning, c.card_type,
  c.tags, c.phonetic, c.example, c.notes, c.created_at,
  s.ease_factor, s.interval_days, s.repetitions, s.lapses, s.due_at,
  s.last_reviewed_at, s.correct_count, s.incorrect_count,
  (s.correct_count + s.incorrect_count) as attempts,
  case when s.correct_count + s.incorrect_count = 0 then 0.0
       else s.incorrect_count::float / (s.correct_count + s.incorrect_count)
  end as error_rate
from cards c
join card_srs s on s.card_id = c.id;
