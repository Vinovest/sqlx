# sqlx

[![Go Coverage](https://github.com/vinovest/sqlx/wiki/coverage.svg)](https://raw.githack.com/wiki/vinovest/sqlx/coverage.html) [![license](https://img.shields.io/badge/license-MIT-green/.svg?style=flat)](https://raw.githubusercontent.com/vinovest/sqlx/master/LICENSE)

This is Vinovest's fork of https://github.com/jmoiron/sqlx.git for
ongoing support. Sqlx hasn't been updated in a while and we use it
extensively, so we're going to maintain this fork with some additional
features and bug fixes.  You're welcome to use and contribute to this
fork!

sqlx is a library which provides a set of extensions on go's standard
`database/sql` library.  The sqlx versions of `sql.DB`, `sql.TX`, `sql.Stmt`,
et al. all leave the underlying interfaces untouched, so that their interfaces
are a superset on the standard ones.  This makes it relatively painless to
integrate existing codebases using database/sql with sqlx.

Major additional concepts are:

* Marshal rows into structs (with embedded struct support), maps, and slices
* Named parameter support including prepared statements
* `Get` and `Select` to go quickly from query to struct/slice
* Bulk insert SQL builder
* Generic functions `One` and `List`
* Range function `AllRows`, or for prepared statements use `All`

In addition to the [godoc API documentation](http://godoc.org/github.com/jmoiron/sqlx),
there is also some [user documentation](http://jmoiron.github.io/sqlx/) that
explains how to use `database/sql` along with sqlx.

## Recent Changes

* Set Go version to 1.23.

* Added `AllRows` package function and `All` on statement types to
  range over rows and automatically close the result set.

* Added functions `One` and `List` for querying with
  generics. Modifications to types Stmt and NamedStmt to support
  generics may cause some small compatibility changes that should be
  quick to resolve with a search and replace. Both are now type
  aliases to attempt to minimize the impact of this change.

* fix for sql in-list parsing that will properly ignore question marks
  in comments and strings. This frequently caused a confusing error
  ("number of bindVars exceeds arguments").

1.5.0:

This is the first major release of sqlx from this fork in order to fix
some issues which may cause some minor compatibility changes from
previous versions, depending on how you use sqlx.

* VALUES bulk insertions were performed by a regex previously, now
  sqlx will use a tokenizer to parse the query and reliably expand the
  query for additional bound parameters. This fixes several bugs
  around named parameters (e.g. `:name`). Where previously a colon had
  to be escaped using an additional colon (resulting in `::name`), now
  the query will work as expected.

* Rebind parsing uses the tokenizer now as well, fixing double quoting
  and unexpected behavior with colons in sql strings.

* Get and GetContext will now error if the query returns more than one
  row. Previously, it fetched the first row and ignored the
  rest. Using Get on multiple rows is typically a mistake, and this
  change makes the behavior more consistent and avoids potential bugs.

* Fix for multiple sql statements ("select a from test; select b from
  test2;") in a single call. Previously, sqlx cached the columns of
  the first query and used them for all subsequent queries. This could
  lead to incorrect results if the queries returned different
  columns. Now, sqlx will reset the columns for each query on
  NextResultSet.

* Adds Queryable interface to unify DB and Tx.

These fixes are the results of upstream contributions submitted in PRs
to the original sqlx repository. Much thanks goes to everyone who
contributed their time and effort to make sqlx better. These fixes
vastly improve the usability of sqlx.

### Backwards Compatibility

Compatibility with the most recent two versions of Go is a requirement for any
new changes.  Compatibility beyond that is not guaranteed.

Versioning is done with Go modules.  Breaking changes (eg. removing deprecated API)
will get major version number bumps.

## Install

    go get github.com/vinovest/sqlx

## Usage

There are several standalone examples in the examples directory that demonstrate various features of sqlx.

* [Generics](./examples/generics/main.go) - query with generics
* [Bulk inserts](./examples/bulk/main.go) - efficient bulk inserts
* [Named queries](./examples/named/main.go) - named parameter queries
* [SQL in-lists](./examples/inlist/main.go) - using `In` to generate SQL `IN` clauses
* [Prepared statements](./examples/preparedstatements/main.go) - prepared statements offer better performance

The simplified form of querying using sqlx looks like the below. Given a struct `Person`, querying all of the rows
in the `person` table is as simple as:

```go
package main

import (
    "fmt"
    
    _ "github.com/lib/pq"
    "github.com/vinovest/sqlx"
)

type Person struct {
    FirstName string `db:"first_name"`
    LastName  string `db:"last_name"`
    Email     string
}

func main() {
    db, _ := sqlx.Connect("postgres", "user=foo dbname=bar sslmode=disable")
    people, _ := sqlx.List[Person](db, "SELECT * FROM person")
    fmt.Printf("People %#v\n", people)

    // alternatively, range over rows
    rows, _ := db.Queryx("select * from person")
    for person, err := range sqlx.AllRows[Person](rows) {
        if err != nil {
            panic(err)
        }
        fmt.Printf("%#v\n", person)
    }
}
```

## Issues

Row headers can be ambiguous (`SELECT 1 AS a, 2 AS a`), and the result of
`Columns()` does not fully qualify column names in queries like:

```sql
SELECT a.id, a.name, b.id, b.name FROM foos AS a JOIN foos AS b ON a.parent = b.id;
```

making a struct or map destination ambiguous.  Use `AS` in your queries
to give columns distinct names, `rows.Scan` to scan them manually, or
`SliceScan` to get a slice of results.
