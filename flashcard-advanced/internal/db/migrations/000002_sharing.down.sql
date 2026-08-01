drop index if exists decks_share_slug_idx;
alter table decks
  drop column if exists share_slug,
  drop column if exists shared_at;
