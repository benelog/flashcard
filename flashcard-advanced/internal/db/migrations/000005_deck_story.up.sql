-- Deck story: a blog-post-like markdown text that gives the deck's cards a
-- shared context (e.g. the full dialogue the expressions were taken from).
alter table decks add column story text;
