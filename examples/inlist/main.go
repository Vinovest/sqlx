package main

// this example illustrates how to use sqlx with in-lists in sql. sqlx has special support for expanding the
// parameter list in a query for you, so you can use the `IN` operator in SQL with a slice of values un the `In` helper.

// alternatively, this also illustrates a better way to handle array data that is postgres-specific using array types
// and the ANY operator. This is a more efficient way to handle array data in postgres, but is not portable to other
// databases.

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/lib/pq"

	"github.com/vinovest/sqlx"
)

// run postgres for this example:
// docker run --name sqlxpg -p 5444:5432 -e POSTGRES_PASSWORD=password -d docker.io/postgres:17.4

const schema = `
	CREATE TABLE IF NOT EXISTS country (
		code text
	);
	TRUNCATE TABLE country;
	insert into country (code) values ('US');
	insert into country (code) values ('UK');
	insert into country (code) values ('FR');
	insert into country (code) values ('asdf');
`

var rollbackError = errors.New("rollback this transaction")

func main() {
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

		// This is not the recommended way to query on postgres, but works on all databases.
		// first step, we need to use In to expand the query into the correct number of placeholders
		query, args, err := sqlx.In(`select code from country where code in (?)`, []string{"US", "UK", "FR"})
		fmt.Println("expanded query:", query) // select code from country where code in (?, ?, ?)
		if err != nil {
			return err
		}
		// then change the expanded query (using dollar sign placeholders) to the correct db placeholders
		query = db.Rebind(query)
		// then we can use Select to get the results
		codes, err := sqlx.List[string](db, query, args...)
		if err != nil {
			return err
		}
		fmt.Printf("codes: %v\n", codes) // [US UK FR]

		// Other parameters work as expected. Pass the entire arguments to In and use `?` placeholders for all arguments.
		query, args, err = sqlx.In(`select code from country where code != ? and code in (?)`, "example", []string{"US", "UK", "FR"})
		fmt.Println("before rebind:", query) // select code from country where code != ? and code in (?, ?, ?)
		query = db.Rebind(query)
		fmt.Println("database query:", query) // select code from country where code != $1 and code in ($2, $3, $4)
		codes, err = sqlx.List[string](db, query, args...)
		if err != nil {
			return err
		}
		fmt.Printf("codes: %v\n", codes) // [US UK FR]

		// alternatively, we can use the ANY operator with array types for a more efficient query and easier syntax.
		// pq.StringArray postgres-specific and does not require manipulating the query string or changing placeholders.
		query = "select code from country where code != $1 and code = any($2)"
		codes, err = sqlx.List[string](db, query, "example", pq.StringArray{"US", "UK", "FR"})
		if err != nil {
			return err
		}
		fmt.Printf("codes: %v\n", codes) // [US UK FR]

		return rollbackError
	})

	if err != nil && !errors.Is(err, rollbackError) {
		panic(err)
	}
}
