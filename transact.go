package sqlx

import "context"

type (
	depthKey           string
	transactionPointer string
)

const (
	dbKeyDepth = depthKey("tradeDBTXDepth")
	txKey      = transactionPointer("transactionPointer")
)

func getTxDepth(ctx context.Context) uint8 {
	v := ctx.Value(dbKeyDepth)
	if v == nil {
		return 0
	}
	return v.(uint8)
}

func getTransactionDB(ctx context.Context) *Tx {
	v := ctx.Value(txKey)
	if v == nil {
		return nil
	}
	return v.(*Tx)
}

func TransactContext(ctx context.Context, db Queryable, txFunc func(context.Context, Queryable) error) error {
	depth := getTxDepth(ctx)
	tx := getTransactionDB(ctx)
	var err error
	if tx != nil {
		// already in a transaction, push down the stack
		ctx = context.WithValue(ctx, dbKeyDepth, depth+1)
	} else if base, ok := db.(*DB); ok {
		tx, err = base.Beginx()
		if err != nil {
			return err
		}
		ctx = context.WithValue(ctx, txKey, tx)
		ctx = context.WithValue(ctx, dbKeyDepth, depth+1)
	}

	defer func() {
		if depth != 0 {
			return
		}

		if p := recover(); p != nil {
			err2 := tx.Rollback()
			if err2 != nil {
				err = err2
			}
			panic(p)
		}

		if ctx.Err() != nil {
			// context was cancelled?
			err = tx.Rollback()
			return
		}

		if err != nil {
			if inErr := tx.Rollback(); inErr != nil {
				err = inErr
			}
		} else {
			err = tx.Commit()
		}
	}()
	err = txFunc(ctx, tx)
	return err
}

func Transact(db Queryable, txFunc func(context.Context, Queryable) error) error {
	return TransactContext(context.Background(), db, txFunc)
}
