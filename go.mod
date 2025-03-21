module github.com/vinovest/sqlx

go 1.22

require (
	github.com/go-sql-driver/mysql v1.9.0
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/muir/sqltoken v0.0.5
)

require filippo.io/edwards25519 v1.1.0 // indirect

replace github.com/muir/sqltoken => github.com/vinovest/sqltoken v0.0.6-sqlx
