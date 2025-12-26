package sqlx

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func oldBindType(driverName string) int {
	switch driverName {
	case "postgres", "pgx", "pq-timeouts", "cloudsqlpostgres", "ql":
		return DOLLAR
	case "mysql":
		return QUESTION
	case "sqlite3":
		return QUESTION
	case "oci8", "ora", "goracle", "godror":
		return NAMED
	case "sqlserver":
		return AT
	}
	return UNKNOWN
}

/*
sync.Map implementation:

goos: linux
goarch: amd64
pkg: github.com/vinovest/sqlx
BenchmarkBindSpeed/old-4         	100000000	        11.0 ns/op
BenchmarkBindSpeed/new-4         	24575726	        50.8 ns/op


async.Value map implementation:

goos: linux
goarch: amd64
pkg: github.com/vinovest/sqlx
BenchmarkBindSpeed/old-4         	100000000	        11.0 ns/op
BenchmarkBindSpeed/new-4         	42535839	        27.5 ns/op
*/

func BenchmarkBindSpeed(b *testing.B) {
	testDrivers := []string{
		"postgres", "pgx", "mysql", "sqlite3", "ora", "sqlserver",
	}

	b.Run("old", func(b *testing.B) {
		b.StopTimer()
		var seq []int
		for i := 0; i < b.N; i++ {
			seq = append(seq, rand.Intn(len(testDrivers)))
		}
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			s := oldBindType(testDrivers[seq[i]])
			if s == UNKNOWN {
				b.Error("unknown driver")
			}
		}

	})

	b.Run("new", func(b *testing.B) {
		b.StopTimer()
		var seq []int
		for i := 0; i < b.N; i++ {
			seq = append(seq, rand.Intn(len(testDrivers)))
		}
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			s := BindType(testDrivers[seq[i]])
			if s == UNKNOWN {
				b.Error("unknown driver")
			}
		}

	})
}

func TestInNotSlice(t *testing.T) {
	now := time.Now()
	args := []any{[]string{"a", "b"}, now}
	query := ` SELECT * FROM person WHERE first_name IN (?) AND id IN ( SELECT id FROM something WHERE created_at = ? )`
	insql, newArgs, err := In(query, args...)
	require.NoError(t, err)
	assert.Contains(t, insql, "IN (?, ?)")
	assert.Contains(t, insql, "WHERE created_at = ? )")
	assert.Equal(t, []any{"a", "b", now}, newArgs)
}
