-- 데이터베이스 기초 장의 연습 문장 모음.
-- 처음부터 다시 시작하고 싶을 때 한 번에 실행한다(기존 연습 표는 지우고 새로 만든다):
--   sqlite3 practice.db < practice.sql

drop table if exists cards;
drop table if exists decks;

-- 장의 실습에서 만드는 연습용 decks 표(2부 최소 앱과 같은 모양)
create table decks (
    id   integer primary key autoincrement,
    name text not null
);

insert into decks (name) values ('토익 필수 단어');

-- alter table 연습: 틀을 고쳐 열을 뒤늦게 더한다
alter table decks add column description text;

-- cards 표와 인덱스(flashcard-basic/schema.sql과 같은 모양)
create table cards (
    id      integer primary key autoincrement,
    deck_id integer not null references decks (id) on delete cascade,
    text    text not null,
    meaning text not null
);

create index cards_deck_idx on cards (deck_id);

insert into cards (deck_id, text, meaning) values (1, 'trade-off', '맞바꿈, 절충');
insert into cards (deck_id, text, meaning) values (1, 'pull request', '코드 반영 요청');

-- 조회 연습: select, 집계, 조인
select text, meaning from cards where deck_id = 1;
select count(*) from cards where deck_id = 1;
select decks.name, cards.text, cards.meaning
from cards
join decks on cards.deck_id = decks.id;
