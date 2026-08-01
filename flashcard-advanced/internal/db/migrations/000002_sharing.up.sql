alter table decks
  add column share_slug text,
  add column shared_at timestamptz;

create unique index decks_share_slug_idx on decks (share_slug)
  where share_slug is not null;
