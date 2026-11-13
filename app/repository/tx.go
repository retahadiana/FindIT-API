package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager struct{ pool *pgxpool.Pool }

func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

func (m *TxManager) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := m.runOnce(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
		if err == nil {
			return nil
		}
		code := pgErrorCode(err)
		if code != codeSerializationFailure && code != codeDeadlockDetected {
			return err
		}
		select {
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.New("transaksi gagal karena konflik konkurensi berulang")
}

func (m *TxManager) WithTxOnce(ctx context.Context, fn func(pgx.Tx) error) error {
	return m.runOnce(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

func (m *TxManager) runOnce(ctx context.Context, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	tx, err := m.pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
