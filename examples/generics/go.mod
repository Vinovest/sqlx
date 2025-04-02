module generics

go 1.23

require (
	github.com/lib/pq v1.10.9
	github.com/vinovest/sqlx v1.5.2
)

require github.com/muir/sqltoken v0.1.0 // indirect

replace github.com/vinovest/sqlx => ../../
