package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// SessionLocker owns a dedicated runtime connection for a nonblocking session advisory lock.
// The callback uses ordinary transactions; the lock connection never occupies the pool.
type SessionLocker interface {
	WithSessionLock(ctx context.Context, key int64, fn func(context.Context) error) (acquired bool, err error)
}

func (r *runner) WithSessionLock(ctx context.Context, key int64, fn func(context.Context) error) (acquired bool, err error) {
	conn, err := pgx.ConnectConfig(ctx, r.pool.Config().ConnConfig.Copy())
	if err != nil {
		return false, MapError(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		return false, MapError(err)
	}
	if !acquired {
		return false, nil
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		var released bool
		unlockErr := conn.QueryRow(unlockCtx, "SELECT pg_advisory_unlock($1)", key).Scan(&released)
		if err == nil {
			if unlockErr != nil {
				err = fmt.Errorf("db: session lock release failed: %w", MapError(unlockErr))
			} else if !released {
				err = fmt.Errorf("db: session lock was not held")
			}
		}
	}()
	return true, fn(ctx)
}
