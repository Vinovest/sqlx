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
	CREATE TABLE IF NOT EXISTS writer (
	    id integer NOT NULL PRIMARY KEY,
		name text
	);
	TRUNCATE TABLE writer;

	CREATE TABLE IF NOT EXISTS book (
	    id integer NOT NULL PRIMARY KEY,
	    name text,
	    -- painfully simplified example
	    author_id integer NOT NULL,
	    coauthor_id integer NOT NULL,
	    reviewer_id integer NOT NULL
	);
	TRUNCATE TABLE book;
	insert into writer (id, name) values (1, 'Jane'), (2, 'McDuff'), (3, 'John');
	insert into book (id, name, author_id, coauthor_id, reviewer_id) values (1, 'Bad neighbors, loud cars', 1, 3, 2);
`

type Writer struct {
	ID   int
	Name string
}

type Book struct {
	ID   int
	Name string

	AuthorID   int    `db:"author_id"`
	CoauthorID int    `db:"coauthor_id"`
	ReviewerID string `db:"reviewer_id"`

	Author   Writer
	Coauthor Writer
	Reviewer Writer
}

var rollbackError = errors.New("rollback this transaction")

func main() {
	db, err := sqlx.Connect("postgres", "user=postgres dbname=postgres password=password port=5444 sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	// starts a transaction that will rollback on error automatically. Shadowing `db` helps to prevent mistakes
	err = sqlx.Transact(db, func(ctx context.Context, db sqlx.Queryable) error {
		db.MustExec(schema)

		// this now works as expected without db tags:
		book, err := sqlx.One[Book](db, `select book.*, author.*, coauthor.*, reviewer.*
			from book
			inner join writer as author
			  on book.author_id = author.id
			inner join writer as coauthor
			  on book.coauthor_id = coauthor.id
			inner join writer as reviewer
			  on book.reviewer_id = reviewer.id
			where book.id = 1`)
		if err != nil {
			return err
		}
		fmt.Println("Got book: ", book.Name)
		fmt.Println("Author: ", book.Author.Name)     // Jane
		fmt.Println("CoAuthor: ", book.Coauthor.Name) // John
		fmt.Println("Reviewer: ", book.Reviewer.Name) // McDuff

		return rollbackError
	})

	if err != nil && !errors.Is(err, rollbackError) {
		panic(err)
	}
}
