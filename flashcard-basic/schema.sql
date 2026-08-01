-- tag::schema[]
create table if not exists decks (
    id   integer primary key autoincrement,
    name text not null
);

create table if not exists cards (
    id      integer primary key autoincrement,
    deck_id integer not null references decks (id) on delete cascade,
    text    text not null,
    meaning text not null
);

create index if not exists cards_deck_idx on cards (deck_id);
-- end::schema[]
