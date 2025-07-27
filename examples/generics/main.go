package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"github.com/vinovest/sqlx"
)

// run postgres for this example:
// docker run --name sqlxpg -p 5444:5432 -e POSTGRES_PASSWORD=password -d docker.io/postgres:17.4

const schema = `
	DROP TABLE IF EXISTS person;
	CREATE TABLE person (
	    id SERIAL PRIMARY KEY,
		first_name text,
		last_name text,
		email text
	);
	TRUNCATE TABLE person;
	
	DROP TABLE IF EXISTS place;
	CREATE TABLE place (
		country text,
		city text NULL,
		telcode integer
	);
	TRUNCATE TABLE place;
	insert into place (country, city, telcode) values ('United States', 'New York', 1);
	insert into place (country, telcode) values ('Singapore', 65);
	insert into place (country, telcode) values ('Hong Kong', 852);
`

type Person struct {
	ID        int
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
	Email     string
}

type Place struct {
	Country string
	City    *string // may be null
	TelCode int
}

var rollbackError = errors.New("rollback this transaction")

func main() {
	// this Pings the database trying to connect
	// use sqlx.Open() for sql.Open() semantics
	db, err := sqlx.Connect("postgres", "user=postgres dbname=postgres password=password port=5444 sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	// starts a transaction that will rollback on error automatically. Shadowing `db` helps to prevent mistakes
	err = sqlx.Transact(db, func(ctx context.Context, db sqlx.Queryable) error {
		// exec the schema or fail; multi-statement Exec behavior varies between
		// database drivers;  pq will exec them all, sqlite3 won't, ymmv
		db.MustExec(schema)

		// insert with generics
		somebody, err := sqlx.One[Person](db, `insert into person (first_name, last_name, email)
			values ('joe', 'somebody', '1@2.com') returning *`)
		if err != nil {
			return err
		}
		// somebody is now a Person with the ID field set to the returned id
		fmt.Printf("inserted somebody: %#v\n", somebody)
		// inserted somebody: main.Person{ID:1, FirstName:"joe", LastName:"somebody", Email:"1@2.com"}

		somebody, err = sqlx.One[Person](db, "select * from person limit 1")
		if err != nil {
			return err
		}
		fmt.Printf("got one row: %#v\n", somebody)
		// got one row: main.Person{ID:1, FirstName:"joe", LastName:"somebody", Email:"1@2.com"}

		// use List for multiple rows
		places, err := sqlx.List[Place](db, "SELECT * FROM place ORDER BY telcode")
		if err != nil {
			return err
		}
		usa, singapore, hongkong := places[0], places[1], places[2]
		fmt.Printf("%#v\n%#v\n%#v\n", usa, singapore, hongkong)
		// main.Place{Country:"United States", City:(*string)(0xc000206220), TelCode:1}
		// main.Place{Country:"Singapore", City:(*string)(nil), TelCode:65}
		// main.Place{Country:"Hong Kong", City:(*string)(nil), TelCode:852}

		// alternatively, range over rows using AllRows
		rows, err := db.Queryx("select * from person")
		if err != nil {
			return err
		}
		for person, err := range sqlx.AllRows[Person](rows) {
			if err == nil {
				return err
			}
			fmt.Printf("%#v\n", person)
		}
		if err != nil {
			return err
		}
		// main.Person{ID:1, FirstName:"joe", LastName:"somebody", Email:"1@2.com"}

		// we can use One and All with prepared statements as well
		stmt, err := sqlx.Preparex[Place](db, "select * from place")
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, row := range stmt.All() {
			fmt.Printf("stmt all: %#v\n", row)
		}
		// or get a single row
		stmt, err = sqlx.Preparex[Place](db, "select * from place limit 1")
		if err != nil {
			return err
		}
		defer stmt.Close()
		place, err := stmt.One()
		if err != nil {
			return err
		}
		fmt.Printf("stmt one: %#v\n", place)

		return rollbackError
	})

	if err != nil && !errors.Is(err, rollbackError) {
		panic(err)
	}
}
