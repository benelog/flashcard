-- 데이터베이스 기초 장의 연습 문장 모음.
-- 원고의 실습을 따라 하다 처음부터 다시 시작하고 싶을 때 한 번에 실행한다:
--   sqlite3 practice.db < practice.sql

create table if not exists decks (id text primary key, name text not null);

insert into decks values ('d1', '토익 필수 단어');

alter table decks add column description text;

select * from decks;
