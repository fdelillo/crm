package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The Tx handed to business code exposes queries and the role switches, and nothing that ends or
// escapes the transaction: fn must not be able to COMMIT, ROLLBACK or reach the connection (INV-03).
func TestTxDoesNotExposeThePgxTransaction(t *testing.T) {
	var value Tx = &tx{}
	if _, ok := value.(pgx.Tx); ok {
		t.Error("the concrete Tx satisfies pgx.Tx: fn could commit, roll back or take the connection")
	}
	if _, ok := value.(interface{ Commit(context.Context) error }); ok {
		t.Error("the concrete Tx has Commit")
	}
	if _, ok := value.(interface{ Rollback(context.Context) error }); ok {
		t.Error("the concrete Tx has Rollback")
	}
	if _, ok := value.(interface{ Conn() *pgx.Conn }); ok {
		t.Error("the concrete Tx has Conn")
	}
	if _, ok := value.(interface {
		Begin(context.Context) (pgx.Tx, error)
	}); ok {
		t.Error("the concrete Tx has Begin (nested transactions)")
	}
	if _, ok := value.(interface {
		SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
	}); ok {
		t.Error("the concrete Tx has SendBatch")
	}
}
