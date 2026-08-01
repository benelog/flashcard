-- tag::schema[]
create table if not exists decks (
    id   integer primary key autoincrement,
    name text not null
);

create table if not exists cards (
    id      integer primary key autoincrement,
    deck_id integer not null references decks (id) on delete cascade,
    front   text not null,
    back    text not null
);
-- end::schema[]
