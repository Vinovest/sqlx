package main

// This example demonstrates a bulk insert using sqlx.

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

		// Often times inserts are performed using question mark placeholders
		result, err := db.NamedExec(`
		INSERT INTO person (first_name, last_name, email)
		-- the sql statement's values clause will be expanded into a bulk insert
		-- ie (?,?),(?,?)
		VALUES (:first_name, :last_name, :email)
`,
			[]map[string]interface{}{
				{
					"first_name": "Bob",
					"last_name":  "Johnson",
					"email":      "bob.johnson@nowhere.co",
				},
				{
					"first_name": "Jane",
					"last_name":  "Doe",
					"email":      "jane.doe@nowhere.co",
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

		// structs work as well
		result, err = db.NamedExec(`
		INSERT INTO person (first_name, last_name, email)
		-- :first_name, :last_name, :email will be expanded into a bulk insert of multiple values
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
		affected, err = result.RowsAffected()
		if err != nil {
			return err
		}
		fmt.Println(fmt.Sprintf("Inserted %v person rows", affected)) // 2

		stmt, err := sqlx.Preparex[Person](db, "SELECT first_name, last_name FROM person")
		if err != nil {
			return err
		}
		for person, err := range stmt.All() {
			if err != nil {
				return err
			}
			fmt.Printf("person: %v\n", person)
		}

		return rollbackError
	})

	if err != nil && !errors.Is(err, rollbackError) {
		panic(err)
	}
}
