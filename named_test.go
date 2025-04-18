package sqlx

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileQuery(t *testing.T) {
	table := []struct {
		d             string
		Q, R, D, T, N string
		V             []string
	}{
		{
			d: "basic test for named parameters, invalid char ',' terminating",
			Q: `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last)`,
			R: `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?)`,
			D: `INSERT INTO foo (a,b,c,d) VALUES ($1, $2, $3, $4)`,
			T: `INSERT INTO foo (a,b,c,d) VALUES (@p1, @p2, @p3, @p4)`,
			N: `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last)`,
			V: []string{"name", "age", "first", "last"},
		},
		{
			d: "This query tests a named parameter ending the string as well as numbers",
			Q: `SELECT * FROM a WHERE first_name=:name1 AND last_name=:name2`,
			R: `SELECT * FROM a WHERE first_name=? AND last_name=?`,
			D: `SELECT * FROM a WHERE first_name=$1 AND last_name=$2`,
			T: `SELECT * FROM a WHERE first_name=@p1 AND last_name=@p2`,
			N: `SELECT * FROM a WHERE first_name=:name1 AND last_name=:name2`,
			V: []string{"name1", "name2"},
		},
		{
			Q: `SELECT ":foo" FROM a WHERE first_name=:name1 AND last_name=:name2`,
			R: `SELECT ":foo" FROM a WHERE first_name=? AND last_name=?`,
			D: `SELECT ":foo" FROM a WHERE first_name=$1 AND last_name=$2`,
			T: `SELECT ":foo" FROM a WHERE first_name=@p1 AND last_name=@p2`,
			N: `SELECT ":foo" FROM a WHERE first_name=:name1 AND last_name=:name2`,
			V: []string{"name1", "name2"},
		},
		{
			Q: `SELECT 'a:b:c' || first_name, '::ABC:_:' FROM person WHERE first_name=:first_name AND last_name=:last_name`,
			R: `SELECT 'a:b:c' || first_name, '::ABC:_:' FROM person WHERE first_name=? AND last_name=?`,
			D: `SELECT 'a:b:c' || first_name, '::ABC:_:' FROM person WHERE first_name=$1 AND last_name=$2`,
			T: `SELECT 'a:b:c' || first_name, '::ABC:_:' FROM person WHERE first_name=@p1 AND last_name=@p2`,
			N: `SELECT 'a:b:c' || first_name, '::ABC:_:' FROM person WHERE first_name=:first_name AND last_name=:last_name`,
			V: []string{"first_name", "last_name"},
		},
		{
			Q: `SELECT @name := "name", :age, :first, :last`,
			R: `SELECT @name := "name", ?, ?, ?`,
			D: `SELECT @name := "name", $1, $2, $3`,
			N: `SELECT @name := "name", :age, :first, :last`,
			T: `SELECT @name := "name", @p1, @p2, @p3`,
			V: []string{"age", "first", "last"},
		},
		{
			Q: `INSERT INTO foo (a,b,c,d) VALUES (:あ, :b, :キコ, :名前)`,
			R: `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?)`,
			D: `INSERT INTO foo (a,b,c,d) VALUES ($1, $2, $3, $4)`,
			N: `INSERT INTO foo (a,b,c,d) VALUES (:あ, :b, :キコ, :名前)`,
			T: `INSERT INTO foo (a,b,c,d) VALUES (@p1, @p2, @p3, @p4)`,
			V: []string{"あ", "b", "キコ", "名前"},
		},
		{
			Q: `SELECT id, added_at::date FROM person WHERE first_name=:first_name AND last_name=:last_name`,
			R: `SELECT id, added_at::date FROM person WHERE first_name=? AND last_name=?`,
			D: `SELECT id, added_at::date FROM person WHERE first_name=$1 AND last_name=$2`,
			T: `SELECT id, added_at::date FROM person WHERE first_name=@p1 AND last_name=@p2`,
			N: `SELECT id, added_at::date FROM person WHERE first_name=:first_name AND last_name=:last_name`,
			V: []string{"first_name", "last_name"},
		},
	}

	for _, test := range table {
		test := test
		n := test.d
		if n == "" {
			n = test.Q
		}
		t.Run(n, func(t *testing.T) {
			if test.d != "" {
				t.Log(test.d)
			}
			t.Log(test.Q)
			compiled, err := compileNamedQuery([]byte(test.Q), QUESTION)
			if err != nil {
				t.Error(err)
			}
			if compiled.query != test.R {
				t.Errorf("R: expected %s, got(R) %s", test.R, compiled.query)
			}
			if len(compiled.names) != len(test.V) {
				t.Errorf("V: expected %#v, got(V) %#v", test.V, compiled.names)
			} else {
				for i, name := range compiled.names {
					if name != test.V[i] {
						t.Errorf("expected %dth name to be %s, got(V) %s", i+1, test.V[i], name)
					}
				}
			}
			compiled, _ = compileNamedQuery([]byte(test.Q), DOLLAR)
			if compiled.query != test.D {
				t.Errorf("\nexpected: `%s`\ngot(D):      `%s`", test.D, compiled.query)
			}

			compiled, _ = compileNamedQuery([]byte(test.Q), AT)
			if compiled.query != test.T {
				t.Errorf("\nexpected: `%s`\ngot(T):      `%s`", test.T, compiled.query)
			}

			compiled, _ = compileNamedQuery([]byte(test.Q), NAMED)
			if compiled.query != test.N {
				t.Errorf("\nexpected: `%s`\ngot(N):      `%s`\n(len: %d vs %d)", test.N, compiled.query, len(test.N), len(compiled.query))
			}
		})
	}
}

type Test struct {
	t *testing.T
}

func (t Test) Error(err error, msg ...interface{}) {
	t.t.Helper()
	if err != nil {
		if len(msg) == 0 {
			t.t.Error(err)
		} else {
			t.t.Error(msg...)
		}
	}
}

func (t Test) Errorf(err error, format string, args ...interface{}) {
	t.t.Helper()
	if err != nil {
		t.t.Errorf(format, args...)
	}
}

func TestEscapedColons(t *testing.T) {
	qs := `SELECT * FROM testtable WHERE timeposted::text BETWEEN (now() AT TIME ZONE 'utc') AND
	(now() AT TIME ZONE 'utc') - interval '01:30:00' AND name = 'this is a test' and id = :id`
	compiled, err := compileNamedQuery([]byte(qs), DOLLAR)
	if err != nil {
		t.Error("Didn't handle colons correctly when inside a string")
	}
	expected := `SELECT * FROM testtable WHERE timeposted::text BETWEEN (now() AT TIME ZONE 'utc') AND
	(now() AT TIME ZONE 'utc') - interval '01:30:00' AND name = 'this is a test' and id = $1`
	if compiled.query != expected {
		t.Errorf("\nexpected: `%s`\ngot(D):      `%s`", expected, compiled.query)
	}
	if len(compiled.names) != 1 {
		t.Errorf("Expected 1 name, got %v", compiled.names)
	}
}

func TestCommentBindName(t *testing.T) {
	qs := `SELECT * FROM testtable -- where 1 = :name`
	compiled, err := compileNamedQuery([]byte(qs), QUESTION)
	if err != nil {
		t.Error("Didn't handle colons correctly when inside a string")
	}
	expected := "SELECT * FROM testtable -- where 1 = :name"
	if compiled.query != expected {
		t.Errorf("\nexpected: `%s`\ngot(D):      `%s`", expected, compiled.query)
	}
	if len(compiled.names) != 0 {
		t.Errorf("Expected no names, got %v", compiled.names)
	}
}

func TestNamedQueries(t *testing.T) {
	RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
		loadDefaultFixture(db, t)
		test := Test{t}
		var ns *NamedStmt
		var err error

		// Check that invalid preparations fail
		_, err = db.PrepareNamed("SELECT * FROM person WHERE first_name=:first:name")
		if err == nil {
			t.Error("Expected an error with invalid prepared statement.")
		}

		_, err = db.PrepareNamed("invalid sql")
		if err == nil {
			t.Error("Expected an error with invalid prepared statement.")
		}

		// Check closing works as anticipated
		ns, err = db.PrepareNamed("SELECT * FROM person WHERE first_name=:first_name")
		test.Error(err)
		err = ns.Close()
		test.Error(err)

		ns, err = db.PrepareNamed(`
			SELECT first_name, last_name, email 
			FROM person WHERE first_name=:first_name AND email=:email`)
		test.Error(err)

		// test Queryx w/ uses Query
		p := Person{FirstName: "Jason", LastName: "Moiron", Email: "jmoiron@jmoiron.net"}

		rows, err := ns.Queryx(p)
		test.Error(err)
		for rows.Next() {
			var p2 Person
			rows.StructScan(&p2)
			if p.FirstName != p2.FirstName {
				t.Errorf("got %s, expected %s", p.FirstName, p2.FirstName)
			}
			if p.LastName != p2.LastName {
				t.Errorf("got %s, expected %s", p.LastName, p2.LastName)
			}
			if p.Email != p2.Email {
				t.Errorf("got %s, expected %s", p.Email, p2.Email)
			}
		}

		// test Select
		people := make([]Person, 0, 5)
		err = ns.Select(&people, p)
		test.Error(err)

		if len(people) != 1 {
			t.Errorf("got %d results, expected %d", len(people), 1)
		}
		if p.FirstName != people[0].FirstName {
			t.Errorf("got %s, expected %s", p.FirstName, people[0].FirstName)
		}
		if p.LastName != people[0].LastName {
			t.Errorf("got %s, expected %s", p.LastName, people[0].LastName)
		}
		if p.Email != people[0].Email {
			t.Errorf("got %s, expected %s", p.Email, people[0].Email)
		}

		// test struct batch inserts
		sls := []Person{
			{FirstName: "Ardie", LastName: "Savea", Email: "asavea@ab.co.nz"},
			{FirstName: "Sonny Bill", LastName: "Williams", Email: "sbw@ab.co.nz"},
			{FirstName: "Ngani", LastName: "Laumape", Email: "nlaumape@ab.co.nz"},
		}

		insert := fmt.Sprintf(
			"INSERT INTO person (first_name, last_name, email, added_at) VALUES (:first_name, :last_name, :email, %v)\n",
			now,
		)
		_, err = db.NamedExec(insert, sls)
		test.Error(err)

		// test map batch inserts
		slsMap := []map[string]interface{}{
			{"first_name": "Ardie", "last_name": "Savea", "email": "asavea@ab.co.nz"},
			{"first_name": "Sonny Bill", "last_name": "Williams", "email": "sbw@ab.co.nz"},
			{"first_name": "Ngani", "last_name": "Laumape", "email": "nlaumape@ab.co.nz"},
		}

		_, err = db.NamedExec(`INSERT INTO person (first_name, last_name, email)
			VALUES (:first_name, :last_name, :email) ;--`, slsMap)
		test.Error(err)

		type A map[string]interface{}

		typedMap := []A{
			{"first_name": "Ardie", "last_name": "Savea", "email": "asavea@ab.co.nz"},
			{"first_name": "Sonny Bill", "last_name": "Williams", "email": "sbw@ab.co.nz"},
			{"first_name": "Ngani", "last_name": "Laumape", "email": "nlaumape@ab.co.nz"},
		}

		_, err = db.NamedExec(`INSERT INTO person (first_name, last_name, email)
			VALUES (:first_name, :last_name, :email) ;--`, typedMap)
		test.Error(err)

		for _, p := range sls {
			dest := Person{}
			err = db.Get(&dest, db.Rebind("SELECT * FROM person WHERE email=? limit 1"), p.Email)
			test.Error(err)
			if dest.Email != p.Email {
				t.Errorf("expected %s, got %s", p.Email, dest.Email)
			}
		}

		// test Exec
		ns, err = db.PrepareNamed(`
			INSERT INTO person (first_name, last_name, email)
			VALUES (:first_name, :last_name, :email)`)
		test.Error(err)

		js := Person{
			FirstName: "Julien",
			LastName:  "Savea",
			Email:     "jsavea@ab.co.nz",
		}
		_, err = ns.Exec(js)
		test.Error(err)

		// Make sure we can pull him out again
		p2 := Person{}
		db.Get(&p2, db.Rebind("SELECT * FROM person WHERE email=?"), js.Email)
		if p2.Email != js.Email {
			t.Errorf("expected %s, got %s", js.Email, p2.Email)
		}

		// test Txn NamedStmts
		tx := db.MustBegin()
		txns := tx.NamedStmt(ns)

		// We're going to add Steven in this txn
		sl := Person{
			FirstName: "Steven",
			LastName:  "Luatua",
			Email:     "sluatua@ab.co.nz",
		}

		_, err = txns.Exec(sl)
		test.Error(err)
		// then rollback...
		tx.Rollback()
		// looking for Steven after a rollback should fail
		err = db.Get(&p2, db.Rebind("SELECT * FROM person WHERE email=?"), sl.Email)
		if err != sql.ErrNoRows {
			t.Errorf("expected no rows error, got %v", err)
		}

		// now do the same, but commit
		tx = db.MustBegin()
		txns = tx.NamedStmt(ns)
		_, err = txns.Exec(sl)
		test.Error(err)
		tx.Commit()

		// looking for Steven after a Commit should succeed
		err = db.Get(&p2, db.Rebind("SELECT * FROM person WHERE email=?"), sl.Email)
		test.Error(err)
		if p2.Email != sl.Email {
			t.Errorf("expected %s, got %s", sl.Email, p2.Email)
		}
	})
}

func TestFixBounds(t *testing.T) {
	table := []struct {
		name, query, expect string
		loop, start, end    int
		expectErr           error
	}{
		{
			name:   `named syntax`,
			query:  `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last)`,
			expect: `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last),(:name, :age, :first, :last)`,
			loop:   2,
			start:  33,
			end:    61,
		},
		{
			name:   `mysql syntax`,
			query:  `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?)`,
			expect: `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?),(?, ?, ?, ?)`,
			loop:   2,
		},
		{
			name:   `named syntax w/ trailer`,
			query:  `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last) ;--`,
			expect: `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last),(:name, :age, :first, :last) ;--`,
			loop:   2,
		},
		{
			name:   `mysql syntax w/ trailer`,
			query:  `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?) ;--`,
			expect: `INSERT INTO foo (a,b,c,d) VALUES (?, ?, ?, ?),(?, ?, ?, ?) ;--`,
			loop:   2,
		},
		{
			name:      `not found test`,
			query:     `INSERT INTO foo (a,b,c,d) (:name, :age, :first, :last)`,
			expect:    `INSERT INTO foo (a,b,c,d) (:name, :age, :first, :last)`,
			expectErr: sql.ErrNoRows,
			loop:      2,
		},
		{
			name:   `found twice test`,
			query:  `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last) VALUES (:name, :age, :first, :last)`,
			expect: `INSERT INTO foo (a,b,c,d) VALUES (:name, :age, :first, :last),(:name, :age, :first, :last) VALUES (:name, :age, :first, :last)`,
			loop:   2,
		},
		{
			name:   `nospace`,
			query:  `INSERT INTO foo (a,b) VALUES(:a, :b)`,
			expect: `INSERT INTO foo (a,b) VALUES(:a, :b),(:a, :b)`,
			loop:   2,
		},
		{
			name:   `lowercase`,
			query:  `INSERT INTO foo (a,b) values(:a, :b)`,
			expect: `INSERT INTO foo (a,b) values(:a, :b),(:a, :b)`,
			loop:   2,
		},
		{
			name:   `on duplicate key using VALUES`,
			query:  `INSERT INTO foo (a,b) VALUES (:a, :b) ON DUPLICATE KEY UPDATE a=VALUES(a)`,
			expect: `INSERT INTO foo (a,b) VALUES (:a, :b),(:a, :b) ON DUPLICATE KEY UPDATE a=VALUES(a)`,
			loop:   2,
		},
		{
			name:   `single column`,
			query:  `INSERT INTO foo (a) VALUES (:a)`,
			expect: `INSERT INTO foo (a) VALUES (:a),(:a)`,
			loop:   2,
		},
		{
			name:   `call now`,
			query:  `INSERT INTO foo (a, b) VALUES (:a, NOW())`,
			expect: `INSERT INTO foo (a, b) VALUES (:a, NOW()),(:a, NOW())`,
			loop:   2,
		},
		{
			name:   `two level depth function call`,
			query:  `INSERT INTO foo (a, b) VALUES (:a, YEAR(NOW()))`,
			expect: `INSERT INTO foo (a, b) VALUES (:a, YEAR(NOW())),(:a, YEAR(NOW()))`,
			loop:   2,
		},
		{
			name:      `missing closing bracket`,
			query:     `INSERT INTO foo (a, b) VALUES (:a, YEAR(NOW())`,
			expect:    `INSERT INTO foo (a, b) VALUES (:a, YEAR(NOW())`,
			expectErr: errors.New("missing closing bracket in VALUES"),
			loop:      2,
		},
		{
			name:   `table with "values" at the end`,
			query:  `INSERT INTO table_values (a, b) VALUES (:a, :b)`,
			expect: `INSERT INTO table_values (a, b) VALUES (:a, :b),(:a, :b)`,
			loop:   2,
		},
		{
			name:   `table named "values"`,
			query:  `INSERT INTO "values" (a, b) VALUES (:a, :b)`,
			expect: `INSERT INTO "values" (a, b) VALUES (:a, :b),(:a, :b)`,
			loop:   2,
		},
		{
			name:   `select from values list`,
			query:  `SELECT * FROM (VALUES (:a, :b))`,
			expect: `SELECT * FROM (VALUES (:a, :b),(:a, :b))`,
			loop:   2,
		},
		{
			name: `multiline indented query`,
			query: `INSERT INTO foo (
		a,
		b,
		c,
		d
	) VALUES (
		:name,
		:age,
		:first,
		:last
	)`,
			expect: `INSERT INTO foo (
		a,
		b,
		c,
		d
	) VALUES (
		:name,
		:age,
		:first,
		:last
	),(
		:name,
		:age,
		:first,
		:last
	)`,
			loop: 2,
		},
		{
			name: "use of VALUES not for insert",
			query: `
UPDATE t
SET
  foo = u.foo,
  bar = u.bar
FROM (
  VALUES (:id, :foo, :bar)
) AS u(id, foo, bar)
WHERE t.id = u.id
`,
			expect: `
UPDATE t
SET
  foo = u.foo,
  bar = u.bar
FROM (
  VALUES (:id, :foo, :bar),(:id, :foo, :bar)
) AS u(id, foo, bar)
WHERE t.id = u.id
`,
			loop: 2,
		},
		{
			name: `multiline WITH query`,
			query: `WITH input_rows(name, age, first, last) AS (
    VALUES (:name, :age, :first, :last)
)
   , ins AS (
    INSERT INTO foo (name, age, first, last)
        SELECT * FROM input_rows
        ON CONFLICT (name, age, first, last) DO NOTHING
        RETURNING id, name, age, first, last
)
SELECT id, name, age, first, last FROM ins
UNION ALL
SELECT id, name, age, first, last
FROM input_rows r JOIN foo c USING (name, age, first, last)`,
			expect: `WITH input_rows(name, age, first, last) AS (
    VALUES (:name, :age, :first, :last),(:name, :age, :first, :last)
)
   , ins AS (
    INSERT INTO foo (name, age, first, last)
        SELECT * FROM input_rows
        ON CONFLICT (name, age, first, last) DO NOTHING
        RETURNING id, name, age, first, last
)
SELECT id, name, age, first, last FROM ins
UNION ALL
SELECT id, name, age, first, last
FROM input_rows r JOIN foo c USING (name, age, first, last)`,
			loop: 2,
		},
		{
			name: `multiline WITH query using casting`,
			query: `WITH input_rows(name, age, first, last) AS (
    VALUES (:name::string, :age::int, :first::string, :last::string)
)
   , ins AS (
    INSERT INTO foo (name, age, first, last)
        SELECT * FROM input_rows
        ON CONFLICT (name, age, first, last) DO NOTHING
        RETURNING id, name, age, first, last
)
SELECT id, name, age, first, last FROM ins
UNION ALL
SELECT id, name, age, first, last
FROM input_rows r JOIN foo c USING (name, age, first, last)`,
			expect: `WITH input_rows(name, age, first, last) AS (
    VALUES (:name::string, :age::int, :first::string, :last::string),(:name::string, :age::int, :first::string, :last::string)
)
   , ins AS (
    INSERT INTO foo (name, age, first, last)
        SELECT * FROM input_rows
        ON CONFLICT (name, age, first, last) DO NOTHING
        RETURNING id, name, age, first, last
)
SELECT id, name, age, first, last FROM ins
UNION ALL
SELECT id, name, age, first, last
FROM input_rows r JOIN foo c USING (name, age, first, last)`,
			loop: 2,
		},
		{
			name: `multiline FROM query using casting`,
			query: `INSERT INTO houses (name, age, address, owner_id)
SELECT i.name, i.age, i.address, p.id
FROM (VALUES (:name, :age, :address, :owner_email)) AS i
	 (name, age, address, email)
	 JOIN people p
		  ON p.email = owner_email;`,
			expect: `INSERT INTO houses (name, age, address, owner_id)
SELECT i.name, i.age, i.address, p.id
FROM (VALUES (:name, :age, :address, :owner_email),(:name, :age, :address, :owner_email)) AS i
	 (name, age, address, email)
	 JOIN people p
		  ON p.email = owner_email;`,
			loop: 2,
		},
		{
			name: `from values`,
			query: `UPDATE
		foo_table f
	SET
		foo = v.foo
	FROM (
		VALUES
		(:id, :foo)
	) AS v ( id, foo )
	WHERE
		f.id = v.id;`,
			expect: `UPDATE
		foo_table f
	SET
		foo = v.foo
	FROM (
		VALUES
		(:id, :foo),(:id, :foo)
	) AS v ( id, foo )
	WHERE
		f.id = v.id;`,
			loop: 2,
		},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			cq, err := compileNamedQuery([]byte(tc.query), NAMED)
			if err != nil {
				if tc.expectErr == nil {
					t.Error(err)
				}
				if tc.expectErr != nil && err.Error() != tc.expectErr.Error() {
					t.Errorf("mismatched error expected %s got %s", tc.expectErr, err)
				}
				return
			}
			fixBound(&cq, tc.loop)
			if cq.query != tc.expect {
				t.Errorf("mismatched results expected:\n%s\ngot:\n%s", tc.expect, cq.query)
			}
			if tc.start > 0 && tc.start != int(*cq.valuesStart) {
				t.Errorf("mismatched start expected %d got %d", tc.start, *cq.valuesStart)
			}
			if tc.end > 0 && tc.end != int(*cq.valuesEnd) {
				t.Errorf("mismatched end expected %d got %d", tc.end, *cq.valuesEnd)
			}
		})
	}
}

func TestAllRows_NamedStmt(t *testing.T) {
	type Simpleton struct {
		FirstName   string `db:"first_name"`
		LastName    string `db:"last_name"`
		Unscannable []string
	}

	testCases := []struct {
		name     string
		sql      string
		expected []Simpleton
		wantErr  string
	}{
		{
			name: "Two rows",
			sql:  "SELECT first_name, last_name FROM person where first_name = :first_name",
			expected: []Simpleton{
				{FirstName: "Jason", LastName: "Moiron"},
			},
		},
		{
			name:     "No rows",
			sql:      "SELECT first_name, last_name FROM person where first_name = :first_name and last_name = 'nope'",
			expected: []Simpleton{},
		},
		{
			name:    "Has struct scan error",
			sql:     "SELECT first_name, 2 as unscannable FROM person",
			wantErr: "sql: Scan error on column index 1, name \"unscannable\": unsupported Scan, storing driver.Value type int64 into type *[]string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RunWithSchema(defaultSchema, t, func(db *DB, t *testing.T, now string) {
				loadDefaultFixture(db, t)
				stmt, err := PrepareNamed[Simpleton](db, tc.sql)
				if err != nil {
					t.Fatalf("Failed to query database: %v", err)
				}

				// Collect the results
				results := make([]Simpleton, 0)
				for person, err := range stmt.All(Simpleton{FirstName: "Jason"}) {
					if tc.wantErr == "" {
						require.NoError(t, err)
					} else if err != nil && tc.wantErr != "" {
						require.Equal(t, tc.wantErr, err.Error())
					}
					results = append(results, person)
				}

				if tc.wantErr == "" {
					assert.Equal(t, tc.expected, results)
				}
			})
		})
	}
}

func BenchmarkFixBound10(b *testing.B) {
	query := `INSERT INTO foo (a,b) VALUES(:a, :b)`
	loop := 10
	b.ReportAllocs()

	cq, err := compileNamedQuery([]byte(query), NAMED)
	if err != nil {
		b.Error(err)
		return
	}
	for i := 0; i < b.N; i++ {
		fixBound(&cq, loop)
	}
}

func BenchmarkFixBound100(b *testing.B) {
	query := `INSERT INTO foo (a,b) VALUES(:a, :b)`
	loop := 100
	b.ReportAllocs()

	cq, err := compileNamedQuery([]byte(query), NAMED)
	if err != nil {
		b.Error(err)
		return
	}
	for i := 0; i < b.N; i++ {
		fixBound(&cq, loop)
	}
}
