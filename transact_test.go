package sqlx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var transactSchema = Schema{
	create: `
CREATE TABLE a (a text);
`,
	drop: `
drop table a;
`,
}

func TestTransact(t *testing.T) {
	var stmtx *Stmt
	var nstmtx *NamedStmt

	tests := []struct {
		name       string
		wantErr    bool
		ctx        func() context.Context
		txFunc     func(context.Context, Queryable) error
		beforeFunc func(context.Context, Queryable)
		afterFunc  func(context.Context, Queryable)
		driverName string
	}{
		{
			name:    "no error",
			wantErr: false,
			ctx:     context.Background,
			txFunc: func(ctx context.Context, db Queryable) error {
				_, err := db.ExecContext(ctx, `INSERT INTO a (a) values ('a3245sdfa')`)
				return err
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				result, err := OneContext[sql.Null[string]](ctx, db, `select a from a WHERE a = 'a3245sdfa'`)
				require.NoError(t, err)
				assert.Equal(t, "a3245sdfa", result.V)
			},
		},
		{
			name:    "no error nested",
			wantErr: false,
			ctx:     context.Background,
			txFunc: func(ctx context.Context, db Queryable) error {
				return TransactContext(ctx, db, func(ctx context.Context, db Queryable) error {
					_, err := db.ExecContext(ctx, `INSERT INTO a (a) values ('a3245sdfa')`)
					return err
				})
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				result, err := OneContext[sql.Null[string]](ctx, db, `select a from a WHERE a = 'a3245sdfa'`)
				require.NoError(t, err)
				assert.Equal(t, "a3245sdfa", result.V)
			},
		},
		{
			name:    "rolls back",
			wantErr: true,
			ctx:     context.Background,
			txFunc: func(ctx context.Context, db Queryable) error {
				_, _ = db.ExecContext(ctx, `INSERT INTO a (a) values ('basdga')`)
				return errors.New("error")
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				_, err := OneContext[string](ctx, db, `select a from a where a = 'basdga'`)
				assert.Error(t, err, "error", "table test value should not exist")
			},
		},
		{
			name:    "rolls back after panic",
			wantErr: true,
			ctx:     context.Background,
			txFunc: func(ctx context.Context, db Queryable) error {
				_, _ = db.ExecContext(ctx, `INSERT INTO a (a) values ('basdga')`)
				panic("ouch")
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				_, err := OneContext[string](ctx, db, `select a from a where a = 'basdga'`)
				assert.Error(t, err, "error", "table test value should not exist")
			},
		},
		{
			name:    "rolls back context error",
			wantErr: false,
			ctx: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now())
				cancel()
				return ctx
			},
			txFunc: func(ctx context.Context, db Queryable) error {
				_, _ = db.ExecContext(ctx, `INSERT INTO a (a) values ('basdga')`)
				return nil
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				_, err := OneContext[string](ctx, db, `select a from a where a = 'basdga'`)
				assert.Error(t, err, "error", "table test value should not exist")
			},
		},
		{
			name: "using a stmt",
			ctx:  context.Background,
			beforeFunc: func(ctx context.Context, db Queryable) {
				var err error
				// prepare a statement on the db
				stmtx, err = db.PreparexContext(ctx, db.Rebind(`INSERT INTO a (a) values (?)`))
				require.NoError(t, err)
			},
			txFunc: func(ctx context.Context, db Queryable) error {
				// to use in the transaction it must be prepared in the transaction
				txstmt := stmtx.Prepare(db)
				_, err := txstmt.ExecContext(ctx, "alpha")
				// txstmt.Close() is not needed, it will be closed when the transaction is committed or rolled back
				require.NoError(t, err)
				return nil
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				f, err := OneContext[string](ctx, db, `select a from a where a = 'alpha'`)
				assert.NoError(t, err)
				assert.Equal(t, "alpha", f)
			},
		},
		{
			name: "using a named stmt",
			ctx:  context.Background,
			beforeFunc: func(ctx context.Context, db Queryable) {
				var err error
				// prepare a statement on the db
				nstmtx, err = db.PrepareNamedContext(ctx, `INSERT INTO a (a) values (:name)`)
				require.NoError(t, err)
			},
			txFunc: func(ctx context.Context, db Queryable) error {
				// to use in the transaction it must be prepared in the transaction
				txstmt := nstmtx.Prepare(db)
				_, err := txstmt.ExecContext(ctx, map[string]any{"name": "aaron"})
				// txstmt.Close() is not needed, it will be closed when the transaction is committed or rolled back
				require.NoError(t, err)
				return nil
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				f, err := OneContext[string](ctx, db, `select a from a where a = 'aaron'`)
				assert.NoError(t, err)
				assert.Equal(t, "aaron", f)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunWithSchema(transactSchema, t, func(db *DB, t *testing.T, now string) {
				var err error
				defer func() {
					e := recover()
					if e != nil {
						if err == nil {
							err = fmt.Errorf("panic: %v", e)
						}
					}
				}()

				if tt.driverName != "" && tt.driverName != db.DriverName() {
					fmt.Println("SKIP test", tt.name, "for driver", db.DriverName())
					return
				}
				fmt.Println("RUN test", tt.name, "for driver", db.DriverName())
				ctx := tt.ctx()
				if tt.beforeFunc != nil {
					tt.beforeFunc(ctx, db)
				}
				if err = TransactContext(ctx, db, tt.txFunc); (err != nil) != tt.wantErr {
					t.Errorf("%v Transact() error = %v, wantErr %v", db.DriverName(), err, tt.wantErr)
				}
				tt.afterFunc(ctx, db)
			})
		})
	}
}
