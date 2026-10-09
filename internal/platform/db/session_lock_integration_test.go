//go:build integration

package db_test

import (
	"context"
	"errors"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/testsupport/pgtest"
	"testing"
)

func TestPhase9SessionLockReleased(t *testing.T) {
	ctx := context.Background()
	locker := db.NewTxRunner(pgtest.AppPool(t)).(db.SessionLocker)
	const key int64 = 0x54455354
	failure := errors.New("callback failed")
	for _, mode := range []string{"error", "cancel", "panic"} {
		t.Run(mode, func(t *testing.T) {
			func() {
				defer func() {
					p := recover()
					if mode == "panic" && p != "test panic" {
						t.Errorf("panic=%v", p)
					}
				}()
				child, cancel := context.WithCancel(ctx)
				defer cancel()
				acquired, err := locker.WithSessionLock(child, key, func(context.Context) error {
					switch mode {
					case "error":
						return failure
					case "cancel":
						cancel()
						return db.ErrCanceled
					case "panic":
						panic("test panic")
					}
					return nil
				})
				if !acquired || err == nil {
					t.Errorf("first acquired=%v err=%v", acquired, err)
				}
			}()
			acquired, err := locker.WithSessionLock(ctx, key, func(context.Context) error { return nil })
			if !acquired || err != nil {
				t.Fatalf("lock leaked acquired=%v err=%v", acquired, err)
			}
		})
	}
}
