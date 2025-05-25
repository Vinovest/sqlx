package sqlx

import (
	"context"
	"database/sql"
	"iter"
)

// A union interface of contextPreparer and binder, required to be able to
// prepare named statements with context (as the bindtype must be determined).
type namedPreparerContext interface {
	PreparerContext
	binder
}

// PrepareNamedContext prepares a named statement for use on the database. Use `PrepareContext` on
// the statement to ready a prepared statement to be used in a transaction.
//
// The returned statement operates within the transaction and will be closed
// when the transaction has been committed or rolled back (you do not need to close it).
func PrepareNamedContext[T any](ctx context.Context, p namedPreparerContext, query string) (*GenericNamedStmt[T], error) {
	bindType := BindType(p.DriverName())
	compiled, err := compileNamedQuery([]byte(query), bindType)
	if err != nil {
		return nil, err
	}
	stmt, err := PreparexContext[T](ctx, p, compiled.query)
	if err != nil {
		return nil, err
	}
	return &GenericNamedStmt[T]{
		QueryString: compiled.query,
		Params:      compiled.names,
		Stmt:        stmt,
	}, nil
}

// PrepareContext returns a transaction-specific prepared statement from
// an existing statement.
//
// It's preferred to use this method over `Prepare` (without context) due to go internals, it
// uses the connection found in context.
//
// The returned statement operates within the transaction and will be closed
// when the transaction has been committed or rolled back (you do not need to close it).
func (n *GenericNamedStmt[T]) PrepareContext(ctx context.Context, ndb Queryable) *GenericNamedStmt[T] {
	tx, ok := ndb.(*Tx)
	if !ok {
		// not needed
		return n
	}
	return &GenericNamedStmt[T]{
		Params:      n.Params,
		QueryString: n.QueryString,
		Stmt: &GenericStmt[T]{
			Stmt:    tx.StmtContext(ctx, n.Stmt.Stmt),
			options: n.Stmt.options,
			Mapper:  n.Stmt.Mapper,
		},
	}
}

// ExecContext executes a named statement using the struct passed.
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) ExecContext(ctx context.Context, arg interface{}) (sql.Result, error) {
	args, err := bindAnyArgs(n.Params, arg, n.Stmt.Mapper)
	if err != nil {
		return *new(sql.Result), err
	}
	return n.Stmt.ExecContext(ctx, args...)
}

// QueryContext executes a named statement using the struct argument, returning rows.
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) QueryContext(ctx context.Context, arg interface{}) (*sql.Rows, error) {
	args, err := bindAnyArgs(n.Params, arg, n.Stmt.Mapper)
	if err != nil {
		return nil, err
	}
	return n.Stmt.QueryContext(ctx, args...)
}

// QueryRowContext executes a named statement against the database.  Because sqlx cannot
// create a *sql.Row with an error condition pre-set for binding errors, sqlx
// returns a *sqlx.Row instead.
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) QueryRowContext(ctx context.Context, arg interface{}) *Row {
	args, err := bindAnyArgs(n.Params, arg, n.Stmt.Mapper)
	if err != nil {
		return &Row{err: err}
	}
	return n.Stmt.QueryRowxContext(ctx, args...)
}

// MustExecContext execs a NamedStmt, panicing on error
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) MustExecContext(ctx context.Context, arg interface{}) sql.Result {
	res, err := n.ExecContext(ctx, arg)
	if err != nil {
		panic(err)
	}
	return res
}

// QueryxContext using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) QueryxContext(ctx context.Context, arg interface{}) (*Rows, error) {
	r, err := n.QueryContext(ctx, arg)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: r, Mapper: n.Stmt.Mapper, options: n.Stmt.options}, err
}

// QueryRowxContext this NamedStmt.  Because of limitations with QueryRow, this is
// an alias for QueryRow.
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) QueryRowxContext(ctx context.Context, arg interface{}) *Row {
	return n.QueryRowContext(ctx, arg)
}

// SelectContext using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) SelectContext(ctx context.Context, dest interface{}, arg interface{}) error {
	rows, err := n.QueryxContext(ctx, arg)
	if err != nil {
		return err
	}
	// if something happens here, we want to make sure the rows are Closed
	defer rows.Close()
	return scanAll(rows, dest, false)
}

// GetContext using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) GetContext(ctx context.Context, dest interface{}, arg interface{}) error {
	r := n.QueryRowxContext(ctx, arg)
	return r.scanAny(dest, false)
}

// OneContext get a single row using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *GenericNamedStmt[T]) OneContext(ctx context.Context, arg interface{}) (T, error) {
	r := n.QueryRowxContext(ctx, arg)
	var dest T
	err := r.scanAny(&dest, false)
	return dest, err
}

// AllContext performs a query using the NamedStmt and returns all rows for use with range.
func (n *GenericNamedStmt[T]) AllContext(ctx context.Context, arg interface{}) iter.Seq2[T, error] {
	rows, err := n.QueryxContext(ctx, arg)
	if err != nil {
		panic(err)
	}

	return func(yield func(T, error) bool) {
		defer func(rows *Rows) {
			_ = rows.Close()
		}(rows)
		for rows.Next() {
			if ctx.Err() != nil {
				return
			}
			var dest T
			err := rows.StructScan(&dest)
			if !yield(dest, err) {
				return
			}
		}
	}
}

// NamedQueryContext binds a named query and then runs Query on the result using the
// provided Ext (sqlx.Tx, sqlx.Db).  It works with both structs and with
// map[string]interface{} types.
func NamedQueryContext(ctx context.Context, e ExtContext, query string, arg interface{}) (*Rows, error) {
	q, args, err := bindNamedMapper(BindType(e.DriverName()), query, arg, mapperFor(e))
	if err != nil {
		return nil, err
	}
	return e.QueryxContext(ctx, q, args...)
}

// NamedExecContext uses BindStruct to get a query executable by the driver and
// then runs Exec on the result.  Returns an error from the binding
// or the query execution itself.
func NamedExecContext(ctx context.Context, e ExtContext, query string, arg interface{}) (sql.Result, error) {
	q, args, err := bindNamedMapper(BindType(e.DriverName()), query, arg, mapperFor(e))
	if err != nil {
		return nil, err
	}
	return e.ExecContext(ctx, q, args...)
}
