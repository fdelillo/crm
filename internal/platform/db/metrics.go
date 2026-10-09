package db

import (
	"expvar"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync/atomic"
)

var metricsPool atomic.Pointer[pgxpool.Pool]

func init() {
	expvar.Publish("set_role_retry_total", expvar.Func(func() any {
		return map[string]int64{RetryRecovered: SetRoleRetryCount(RetryRecovered), RetryFailed: SetRoleRetryCount(RetryFailed)}
	}))
	// Stat reads the pool's counters without acquiring a connection or querying PostgreSQL.
	expvar.Publish("db_pool_acquire_wait_ms", expvar.Func(func() any {
		pool := metricsPool.Load()
		if pool == nil {
			return float64(0)
		}
		return float64(pool.Stat().AcquireDuration().Microseconds()) / 1000
	}))
}
