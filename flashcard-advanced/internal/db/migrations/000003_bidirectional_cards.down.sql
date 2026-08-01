-- Note: the 'concept' value added to card_type cannot be removed from the
-- enum; it is left in place (rows using it would block a true rollback).

drop view cards_with_stats;

alter table cards rename column text to front_text;
alter table cards rename column meaning to back_text;

alter table study_sessions drop column direction;
drop type study_direction;

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
