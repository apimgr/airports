package db

import (
	"context"
	"database/sql"
	"errors"
)

// Handle is the SQL surface shared by database/sql connections and Store.
// Production code receives Store, while tests may use an isolated *sql.DB.
type Handle interface {
	Close() error
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PingContext(ctx context.Context) error
	Stats() sql.DBStats
}

var _ Handle = (*sql.DB)(nil)
var _ Handle = (*Store)(nil)

// Transaction runs a bounded transaction through Store.Write when available.
// A raw database/sql handle remains supported for isolated compatibility tests.
func Transaction(ctx context.Context, database Handle, fn func(*TransactionHandle) error) error {
	if database == nil {
		return errors.New("db: transaction database handle is nil")
	}
	if fn == nil {
		return errors.New("db: nil transaction callback")
	}
	if store, ok := database.(*Store); ok {
		return store.Write(ctx, func(tx *sql.Tx) error {
			return fn(&TransactionHandle{tx: tx, remote: store.remote})
		})
	}
	ctx, cancel := context.WithTimeout(ctx, transactionTimeout)
	defer cancel()
	transactional, ok := database.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return errors.New("db: transaction handle does not support transactions")
	}
	tx, err := transactional.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(&TransactionHandle{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

// TransactionHandle is a transaction whose SQL statements receive remote name mapping.
type TransactionHandle struct {
	tx     *sql.Tx
	remote bool
}

func (tx *TransactionHandle) Exec(query string, args ...any) (sql.Result, error) {
	return tx.tx.Exec(rewriteTableNames(query, tx.remote), args...)
}

func (tx *TransactionHandle) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, rewriteTableNames(query, tx.remote), args...)
}

func (tx *TransactionHandle) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.tx.Query(rewriteTableNames(query, tx.remote), args...)
}

func (tx *TransactionHandle) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return tx.tx.QueryContext(ctx, rewriteTableNames(query, tx.remote), args...)
}

func (tx *TransactionHandle) QueryRow(query string, args ...any) *sql.Row {
	return tx.tx.QueryRow(rewriteTableNames(query, tx.remote), args...)
}

func (tx *TransactionHandle) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.tx.QueryRowContext(ctx, rewriteTableNames(query, tx.remote), args...)
}
