package sqlx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

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
	tests := []struct {
		name       string
		wantErr    bool
		ctx        context.Context
		txFunc     func(context.Context, Queryable) error
		afterFunc  func(context.Context, Queryable)
		driverName string
	}{
		{
			name:    "no error",
			wantErr: false,
			ctx:     context.Background(),
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
			name:    "rolls back",
			wantErr: true,
			ctx:     context.Background(),
			txFunc: func(ctx context.Context, db Queryable) error {
				_, _ = db.ExecContext(ctx, `INSERT INTO a (a) values ('basdga')`)
				return errors.New("error")
			},
			afterFunc: func(ctx context.Context, db Queryable) {
				_, err := OneContext[string](ctx, db, `select a from a where a = 'basdga'`)
				assert.Error(t, err, "error", "table test should not exist")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunWithSchema(transactSchema, t, func(db *DB, t *testing.T, now string) {
				if tt.driverName != "" && tt.driverName != db.DriverName() {
					fmt.Println("SKIP test", tt.name, "for driver", db.DriverName())
					return
				}
				fmt.Println("RUN test", tt.name, "for driver", db.DriverName())
				if err := TransactContext(tt.ctx, db, tt.txFunc); (err != nil) != tt.wantErr {
					t.Errorf("%v Transact() error = %v, wantErr %v", db.DriverName(), err, tt.wantErr)
				}
				tt.afterFunc(tt.ctx, db)
			})
		})
	}
}
