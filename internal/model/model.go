// Package model holds what the whole app agrees on: the row types that travel
// between layers, the sentinel error, the values each column may take, and the
// small pure functions that go with them.
//
// 데이터베이스도 HTTP도 모르는 패키지다. 그래서 저장소 구현 둘(pgx를 쓰는
// internal/pgstore, SQLite를 쓰는 internal/litestore)이 나란히 이것을 의존할 수
// 있고, 화면(internal/web)과 JSON API(internal/handlers), CSV 변환
// (internal/cardcsv)도 드라이버를 끌고 오지 않고 카드와 덱을 다룰 수 있다.
package model

import "errors"

// tag::not-found[]
var ErrNotFound = errors.New("not found")

// end::not-found[]
