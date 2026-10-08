package tenant

import (
	"context"
	"fmt"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/tenant/store"
	"github.com/google/uuid"
	"strings"
)

// ReprovisionLockKey is a stable cluster-wide key for this operation (DD-33 R-d).
const ReprovisionLockKey int64 = 0x43524d525052

type ReprovisionReport struct {
	Tenants int      `json:"tenants"`
	Failed  []string `json:"failed"`
}

// Reprovision repairs role memberships with one committed transaction per company.
func (s *Service) Reprovision(ctx context.Context) (ReprovisionReport, error) {
	report := ReprovisionReport{Failed: []string{}}
	locker, ok := s.runner.(db.SessionLocker)
	if !ok {
		return report, fmt.Errorf("tenant: runner does not support session locks")
	}
	acquired, err := locker.WithSessionLock(ctx, ReprovisionLockKey, func(ctx context.Context) error {
		var ids []uuid.UUID
		if err := s.runner.InSystemTx(ctx, db.RoleWorker, func(ctx context.Context, tx db.Tx) error {
			var err error
			ids, err = store.New(tx).ListTenantIDs(ctx)
			return db.MapError(err)
		}); err != nil {
			return err
		}
		for _, id := range ids {
			if ctx.Err() != nil {
				return db.MapError(ctx.Err())
			}
			report.Tenants++
			err := s.runner.InSystemTx(ctx, db.RoleSignup, func(ctx context.Context, tx db.Tx) error {
				_, err := store.New(tx).ProvisionTenantRole(ctx, id)
				return db.MapError(err)
			})
			if err != nil {
				if ctx.Err() != nil {
					return db.MapError(ctx.Err())
				}
				report.Failed = append(report.Failed, id.String())
				s.logger.ErrorContext(ctx, "company reprovision failed", "tenant_id", id, "err", err)
			}
		}
		if len(report.Failed) > 0 {
			return fmt.Errorf("reprovisión falló para: %s", strings.Join(report.Failed, ", "))
		}
		return nil
	})
	if err != nil {
		return report, err
	}
	if !acquired {
		return report, fmt.Errorf("otra reprovisión en curso")
	}
	return report, nil
}
