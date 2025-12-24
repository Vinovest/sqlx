package sqlx

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// ConnectContext to a database and verify with a ping.
func ConnectContext(ctx context.Context, driverName, dataSourceName string) (*DB, error) {
	db, err := Open(driverName, dataSourceName)
	if err != nil {
		return db, err
	}
	err = db.PingContext(ctx)
	return db, err
}

// QueryerContext is an interface used by GetContext and SelectContext
type QueryerContext interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryxContext(ctx context.Context, query string, args ...any) (*Rows, error)
	QueryRowxContext(ctx context.Context, query string, args ...any) *Row
}

// PreparerContext is an interface used by PreparexContext.
type PreparerContext interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// ExecerContext is an interface used by MustExecContext and LoadFileContext
type ExecerContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ExtContext is a union interface which can bind, query, and exec, with Context
// used by NamedQueryContext and NamedExecContext.
type ExtContext interface {
	binder
	QueryerContext
	ExecerContext
}

// SelectContext executes a query using the provided Queryer, and StructScans
// each row into dest, which must be a slice.  If the slice elements are
// scannable, then the result set must have only one column.  Otherwise,
// StructScan is used. The *sql.Rows are closed automatically.
// Any placeholder parameters are replaced with supplied args.
func SelectContext(ctx context.Context, q QueryerContext, dest any, query string, args ...any) error {
	rows, err := q.QueryxContext(ctx, query, args...)
	if err != nil {
		return err
	}
	// if something happens here, we want to make sure the rows are Closed
	defer rows.Close()
	return scanAll(rows, dest, false)
}

// PreparexContext prepares a statement.
//
// The provided context is used for the preparation of the statement, not for
// the execution of the statement.
func PreparexContext[T any](ctx context.Context, p PreparerContext, query string) (*GenericStmt[T], error) {
	s, err := p.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &GenericStmt[T]{Stmt: s, options: getOptions(p), Mapper: mapperFor(p)}, err
}

// preparexContextStmt returns a Stmt until type aliases support generics. It sounds like PreparexContext[any] returning Stmt will work in 1.24
func preparexContextStmt(ctx context.Context, p PreparerContext, query string) (*Stmt, error) {
	s, err := p.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &Stmt{Stmt: s, options: getOptions(p), Mapper: mapperFor(p)}, err
}

// GetContext does a QueryRow using the provided Queryer, and scans the
// resulting row to dest.  If dest is scannable, the result must only have one
// column. Otherwise, StructScan is used.  Get will return sql.ErrNoRows like
// row.Scan would. Any placeholder parameters are replaced with supplied args.
// An error is returned if the result set is empty.
func GetContext(ctx context.Context, q QueryerContext, dest any, query string, args ...any) error {
	r := q.QueryRowxContext(ctx, query, args...)
	return r.scanAny(dest, false)
}

// OneContext does a QueryRow using the provided Queryer, and scans the
// resulting row to dest.  If dest is scannable, the result must only have one
// column. Otherwise, StructScan is used.  Get will return sql.ErrNoRows like
// row.Scan would. Any placeholder parameters are replaced with supplied args.
// An error is returned if the result set is empty.
func OneContext[T any](ctx context.Context, q QueryerContext, query string, args ...any) (T, error) {
	var dest T
	r := q.QueryRowxContext(ctx, query, args...)
	err := r.scanAny(&dest, false)
	return dest, err
}

// ListContext executes a query using the provided Queryer, and returns a slice of T for each row.
func ListContext[T any](ctx context.Context, q QueryerContext, query string, args ...any) ([]T, error) {
	var dest []T
	err := SelectContext(ctx, q, &dest, query, args...)
	return dest, err
}

// LoadFileContext exec's every statement in a file (as a single call to Exec).
// LoadFileContext may return a nil *sql.Result if errors are encountered
// locating or reading the file at path.  LoadFile reads the entire file into
// memory, so it is not suitable for loading large data dumps, but can be useful
// for initializing schemas or loading indexes.
//
// FIXME: this does not really work with multi-statement files for mattn/go-sqlite3
// or the go-mysql-driver/mysql drivers;  pq seems to be an exception here.  Detecting
// this by requiring something with DriverName() and then attempting to split the
// queries will be difficult to get right, and its current driver-specific behavior
// is deemed at least not complex in its incorrectness.
func LoadFileContext(ctx context.Context, e ExecerContext, path string) (*sql.Result, error) {
	realpath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(realpath)
	if err != nil {
		return nil, err
	}
	res, err := e.ExecContext(ctx, string(contents))
	return &res, err
}

// MustExecContext execs the query using e and panics if there was an error.
// Any placeholder parameters are replaced with supplied args.
func MustExecContext(ctx context.Context, e ExecerContext, query string, args ...any) sql.Result {
	res, err := e.ExecContext(ctx, query, args...)
	if err != nil {
		panic(err)
	}
	return res
}

// PrepareNamedContext returns an sqlx.NamedStmt
func (db *DB) PrepareNamedContext(ctx context.Context, query string) (*NamedStmt, error) {
	return PrepareNamedContext[any](ctx, db, query)
}

// NamedQueryContext using this DB.
// Any named placeholder parameters are replaced with fields from arg.
func (db *DB) NamedQueryContext(ctx context.Context, query string, arg any) (*Rows, error) {
	return NamedQueryContext(ctx, db, query, arg)
}

// NamedExecContext using this DB.
// Any named placeholder parameters are replaced with fields from arg.
func (db *DB) NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error) {
	return NamedExecContext(ctx, db, query, arg)
}

// SelectContext using this DB.
// Any placeholder parameters are replaced with supplied args.
func (db *DB) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	return SelectContext(ctx, db, dest, query, args...)
}

// GetContext using this DB.
// Any placeholder parameters are replaced with supplied args.
// An error is returned if the result set is empty.
func (db *DB) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	return GetContext(ctx, db, dest, query, args...)
}

// PreparexContext returns an sqlx.Stmt instead of a sql.Stmt.
//
// The provided context is used for the preparation of the statement, not for
// the execution of the statement.
func (db *DB) PreparexContext(ctx context.Context, query string) (*Stmt, error) {
	return preparexContextStmt(ctx, db, query)
}

// QueryxContext queries the database and returns an *sqlx.Rows.
// Any placeholder parameters are replaced with supplied args.
func (db *DB) QueryxContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: r, options: db.options, Mapper: db.Mapper}, err
}

// QueryRowxContext queries the database and returns an *sqlx.Row.
// Any placeholder parameters are replaced with supplied args.
func (db *DB) QueryRowxContext(ctx context.Context, query string, args ...any) *Row {
	rows, err := db.DB.QueryContext(ctx, query, args...)
	return &Row{rows: rows, err: err, options: db.options, Mapper: db.Mapper}
}

// MustBeginTx starts a transaction, and panics on error.  Returns an *sqlx.Tx instead
// of an *sql.Tx.
//
// The provided context is used until the transaction is committed or rolled
// back. If the context is canceled, the sql package will roll back the
// transaction. Tx.Commit will return an error if the context provided to
// MustBeginContext is canceled.
func (db *DB) MustBeginTx(ctx context.Context, opts *sql.TxOptions) *Tx {
	tx, err := db.BeginTxx(ctx, opts)
	if err != nil {
		panic(err)
	}
	return tx
}

// MustExecContext (panic) runs MustExec using this database.
// Any placeholder parameters are replaced with supplied args.
func (db *DB) MustExecContext(ctx context.Context, query string, args ...any) sql.Result {
	return MustExecContext(ctx, db, query, args...)
}

// BeginTxx begins a transaction and returns an *sqlx.Tx instead of an
// *sql.Tx.
//
// The provided context is used until the transaction is committed or rolled
// back. If the context is canceled, the sql package will roll back the
// transaction. Tx.Commit will return an error if the context provided to
// BeginxContext is canceled.
func (db *DB) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, driverName: db.driverName, options: db.options, Mapper: db.Mapper}, err
}

// Connx returns an *sqlx.Conn instead of an *sql.Conn.
func (db *DB) Connx(ctx context.Context) (*Conn, error) {
	conn, err := db.DB.Conn(ctx)
	if err != nil {
		return nil, err
	}

	return &Conn{Conn: conn, driverName: db.driverName, options: db.options, Mapper: db.Mapper}, nil
}

// BeginTxx begins a transaction and returns an *sqlx.Tx instead of an
// *sql.Tx.
//
// The provided context is used until the transaction is committed or rolled
// back. If the context is canceled, the sql package will roll back the
// transaction. Tx.Commit will return an error if the context provided to
// BeginxContext is canceled.
func (c *Conn) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := c.Conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, driverName: c.driverName, options: c.options, Mapper: c.Mapper}, err
}

// SelectContext using this Conn.
// Any placeholder parameters are replaced with supplied args.
func (c *Conn) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	return SelectContext(ctx, c, dest, query, args...)
}

// GetContext using this Conn.
// Any placeholder parameters are replaced with supplied args.
// An error is returned if the result set is empty.
func (c *Conn) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	return GetContext(ctx, c, dest, query, args...)
}

// PreparexContext returns an sqlx.Stmt instead of a sql.Stmt.
//
// The provided context is used for the preparation of the statement, not for
// the execution of the statement.
func (c *Conn) PreparexContext(ctx context.Context, query string) (*Stmt, error) {
	return preparexContextStmt(ctx, c, query)
}

// QueryxContext queries the database and returns an *sqlx.Rows.
// Any placeholder parameters are replaced with supplied args.
func (c *Conn) QueryxContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := c.Conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: r, options: c.options, Mapper: c.Mapper}, err
}

// QueryRowxContext queries the database and returns an *sqlx.Row.
// Any placeholder parameters are replaced with supplied args.
func (c *Conn) QueryRowxContext(ctx context.Context, query string, args ...any) *Row {
	rows, err := c.Conn.QueryContext(ctx, query, args...)
	return &Row{rows: rows, err: err, options: c.options, Mapper: c.Mapper}
}

// Rebind a query within a Conn's bindvar type.
func (c *Conn) Rebind(query string) string {
	return Rebind(BindType(c.driverName), query)
}

// StmtxContext returns a version of the prepared statement which runs within a
// transaction. Provided stmt can be either *sql.Stmt or *sqlx.Stmt.
func (tx *Tx) StmtxContext(ctx context.Context, stmt any) *GenericStmt[any] {
	var s *sql.Stmt
	switch v := stmt.(type) {
	case Stmt:
		s = v.Stmt
	case *Stmt:
		s = v.Stmt
	case *sql.Stmt:
		s = v
	default:
		panic(fmt.Sprintf("non-statement type %v passed to Stmtx", reflect.ValueOf(stmt).Type()))
	}
	return &GenericStmt[any]{Stmt: tx.StmtContext(ctx, s), Mapper: tx.Mapper, options: tx.options}
}

// NamedStmtContext returns a version of the prepared statement which runs
// within a transaction.
func (tx *Tx) NamedStmtContext(ctx context.Context, stmt *NamedStmt) *NamedStmt {
	return &NamedStmt{
		QueryString: stmt.QueryString,
		Params:      stmt.Params,
		Stmt:        tx.StmtxContext(ctx, stmt.Stmt),
	}
}

// PreparexContext returns an sqlx.Stmt instead of a sql.Stmt.
//
// The provided context is used for the preparation of the statement, not for
// the execution of the statement.
func (tx *Tx) PreparexContext(ctx context.Context, query string) (*Stmt, error) {
	return preparexContextStmt(ctx, tx, query)
}

// PrepareNamedContext returns an sqlx.NamedStmt
func (tx *Tx) PrepareNamedContext(ctx context.Context, query string) (*NamedStmt, error) {
	return PrepareNamedContext[any](ctx, tx, query)
}

// MustExecContext runs MustExecContext within a transaction.
// Any placeholder parameters are replaced with supplied args.
func (tx *Tx) MustExecContext(ctx context.Context, query string, args ...any) sql.Result {
	return MustExecContext(ctx, tx, query, args...)
}

// QueryxContext within a transaction and context.
// Any placeholder parameters are replaced with supplied args.
func (tx *Tx) QueryxContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := tx.Tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: r, options: tx.options, Mapper: tx.Mapper}, err
}

// SelectContext within a transaction and context.
// Any placeholder parameters are replaced with supplied args.
func (tx *Tx) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	return SelectContext(ctx, tx, dest, query, args...)
}

// GetContext within a transaction and context.
// Any placeholder parameters are replaced with supplied args.
// An error is returned if the result set is empty.
func (tx *Tx) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	return GetContext(ctx, tx, dest, query, args...)
}

// QueryRowxContext within a transaction and context.
// Any placeholder parameters are replaced with supplied args.
func (tx *Tx) QueryRowxContext(ctx context.Context, query string, args ...any) *Row {
	rows, err := tx.Tx.QueryContext(ctx, query, args...)
	return &Row{rows: rows, err: err, options: tx.options, Mapper: tx.Mapper}
}

// NamedExecContext using this Tx.
// Any named placeholder parameters are replaced with fields from arg.
func (tx *Tx) NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error) {
	return NamedExecContext(ctx, tx, query, arg)
}

// SelectContext using the prepared statement.
// Any placeholder parameters are replaced with supplied args.
func (s *GenericStmt[T]) SelectContext(ctx context.Context, dest any, args ...any) error {
	return SelectContext(ctx, &qStmt[T]{s}, dest, "", args...)
}

// GetContext using the prepared statement.
// Any placeholder parameters are replaced with supplied args.
// An error is returned if the result set is empty.
func (s *GenericStmt[T]) GetContext(ctx context.Context, dest any, args ...any) error {
	return GetContext(ctx, &qStmt[T]{s}, dest, "", args...)
}

// MustExecContext (panic) using this statement.  Note that the query portion of
// the error output will be blank, as Stmt does not expose its query.
// Any placeholder parameters are replaced with supplied args.
func (s *GenericStmt[T]) MustExecContext(ctx context.Context, args ...any) sql.Result {
	return MustExecContext(ctx, &qStmt[T]{s}, "", args...)
}

// QueryRowxContext using this statement.
// Any placeholder parameters are replaced with supplied args.
func (s *GenericStmt[T]) QueryRowxContext(ctx context.Context, args ...any) *Row {
	qs := &qStmt[T]{s}
	return qs.QueryRowxContext(ctx, "", args...)
}

// QueryxContext using this statement.
// Any placeholder parameters are replaced with supplied args.
func (s *GenericStmt[T]) QueryxContext(ctx context.Context, args ...any) (*Rows, error) {
	qs := &qStmt[T]{s}
	return qs.QueryxContext(ctx, "", args...)
}

func (q *qStmt[T]) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.Stmt.QueryContext(ctx, args...)
}

func (q *qStmt[T]) QueryxContext(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := q.Stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: r, options: q.Stmt.options, Mapper: q.Stmt.Mapper}, err
}

func (q *qStmt[T]) QueryRowxContext(ctx context.Context, query string, args ...any) *Row {
	rows, err := q.Stmt.QueryContext(ctx, args...)
	return &Row{rows: rows, err: err, options: q.Stmt.options, Mapper: q.Stmt.Mapper}
}

func (q *qStmt[T]) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.Stmt.ExecContext(ctx, args...)
}
