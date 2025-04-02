package main

import (
	"context"
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"github.com/vinovest/sqlx"
)

// Using prepared statements is essential for performance and security.
// This demonstrates the usage of prepared statements with sqlx. Usage is generally the same as
// using the standard library, but named statements in particular benefit from caching to avoid
// the overhead of re-parsing.

// run postgres for this example:
// docker run --name sqlxpg -p 5444:5432 -e POSTGRES_PASSWORD=password -d docker.io/postgres:17.4

const schema = `
	CREATE TABLE IF NOT EXISTS contacts (
	    id serial PRIMARY KEY,
		first_name text,
		last_name text
	);
	TRUNCATE TABLE contacts;

	insert into contacts (first_name, last_name) values ('joe', 'bob');
	insert into contacts (first_name, last_name) values ('barbara', 'joe');
	insert into contacts (first_name, last_name) values ('joe', 'dirt');
	insert into contacts (first_name) values ('joey');
`

const cleanup = `
	drop table contacts;
`

type Contact struct {
	ID        int64  `db:"id"`
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
}

func main() {
	// this Pings the database trying to connect
	// use sqlx.Open() for sql.Open() semantics
	db, err := sqlx.Connect("postgres", "user=postgres dbname=postgres password=password port=5444 sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	db.MustExec(schema)

	// prepare the statement here. Internally, sqlx parses the sql to find :first_name and creates a struct
	// containing the resulting query string for the `db` driver and expected parameters.
	stmt, err := sqlx.PrepareNamed[Contact](db, "SELECT first_name, last_name FROM contacts WHERE first_name = :first_name")
	if err != nil {
		panic(err)
	}
	fmt.Println(stmt.Params) // [first_name]

	// starts a transaction that will rollback on error automatically. Shadowing `db` helps to prevent mistakes
	err = sqlx.Transact(db, func(ctx context.Context, db sqlx.Queryable) error {
		// To use the statement in the transaction using the current connection, use Prepare.
		// If `db` is not a transaction, this is a no-op so it's safe to use every time.
		txStmt := stmt.Prepare(db)
		contacts, err := txStmt.List(map[string]interface{}{"first_name": "joe"})
		if err != nil {
			return err
		}
		fmt.Println("in transaction", contacts) // prints 2 contacts with first name "joe"

		// in a transaction we could still use the stmt we prepared above but this might result in unexpected
		// behavior. It executes the query on a new connection. But since we're in a transaction, `db` is
		// really a *Tx.
		// to demonstrate this, let's remove all contacts:
		_, err = db.Exec("DELETE FROM contacts")
		if err != nil {
			return err
		}

		// now let's use the outer stmt to get the contacts again
		contacts, err = stmt.List(map[string]interface{}{"first_name": "joe"})
		if err != nil {
			return err
		}
		fmt.Println("new connection", contacts) // prints 2 contacts with first name "joe"

		return nil
	})

	db.MustExec(cleanup)
	if err != nil {
		panic(err)
	}
}
