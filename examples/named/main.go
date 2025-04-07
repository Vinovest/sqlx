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
	CREATE TABLE IF NOT EXISTS person (
		first_name text,
		last_name text,
		email text
	);
	TRUNCATE TABLE person;
	
	CREATE TABLE IF NOT EXISTS place (
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
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
	Email     string
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

		result, err := db.NamedExec(`
		INSERT INTO person (first_name, last_name, email)
		-- these named parameters will be expanded into a bulk insert of multiple values
		VALUES (:first_name, :last_name, :email)
`,
			[]Person{
				{
					FirstName: "Jason",
					LastName:  "Moiron",
					Email:     "jmoiron@jmoiron.net",
				},
				{
					FirstName: "John",
					LastName:  "Doe",
					Email:     "johndoeDNE@gmail.net",
				},
			})
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		fmt.Println(fmt.Sprintf("Inserted %v person rows", affected)) // 2

		// Named queries, using `:name` as the bindvar.  Automatic bindvar support
		// which takes into account the dbtype based on the driverName on sqlx.Open/Connect
		_, err = db.NamedExec(`
			INSERT INTO person (first_name, last_name, email)
			VALUES (:first, :last, :email)
		`,
			map[string]interface{}{
				"first": "Bin",
				"last":  "Smuth",
				"email": "bensmith@ex.co",
			})

		// Selects Mr. Smith from the database using a map of string parameters. This also works with structs.
		rows, err := db.NamedQuery(`SELECT * FROM person WHERE first_name = :fn`, map[string]interface{}{"fn": "Bin"})
		if err != nil {
			return err
		}
		for row := range sqlx.AllRows[Person](rows) {
			fmt.Printf("Mr Smith: %#v\n", row)
		}

		// Using named parameters with functions like `Select` does not work. Use a Named
		// function so sqlx will perform the placeholder conversion.
		//   will cause an error: db.Select(&[]Person{}, `SELECT * FROM person WHERE first_name = :fn`, map[string]interface{}{"fn": "Bin"})
		// You can instead use the prepared statement to perform the same query. This is optional
		// but can increase performance if caching the statement for future use, it will avoid
		// the overhead of preparing the sql each time.
		personQuery, err := sqlx.PrepareNamed[Person](db, "SELECT first_name, last_name FROM person WHERE first_name = :first_name")
		if err != nil {
			return err
		}
		// this uses the generic method available on the NamedGenericStmt struct created through the PrepareNamed function
		person, err := personQuery.One(map[string]interface{}{"first_name": "Bin"})
		if err != nil {
			return err
		}
		fmt.Printf("Mr Smith (named prepared statement): %#v\n", person)
		// you can also do the same query using a struct for filter parameters:
		person, err = personQuery.One(Person{FirstName: "Jason"})
		if err != nil {
			return err
		}
		fmt.Printf("Mr Smith (using struct parameters): %#v\n", person)
		// query existing person using struct for filtering and All func iterator
		for person, err := range personQuery.All(person) {
			if err != nil {
				return err
			}
			fmt.Printf("Mr Smith (from func iterator): %#v\n", person)
		}

		return rollbackError
	})

	if err != nil && !errors.Is(err, rollbackError) {
		panic(err)
	}
}
