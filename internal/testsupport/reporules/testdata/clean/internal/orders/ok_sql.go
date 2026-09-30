package orders

import (
	"context"
	"net/http"
	"strings"
)

type conn interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) error
}

type batch struct{}

func (b *batch) Queue(sql string, args ...any) {}

type jobs struct{}

func (j *jobs) Queue(job string) {}

// Constants (as sqlc generates them) and literals are the only SQL a call may receive.
const listQ = "SELECT id FROM app.t WHERE tenant_id = $1"

const (
	insertQ = `INSERT INTO app.t (id) VALUES ($1)`
)

func good(ctx context.Context, c conn, b *batch) error {
	_ = c.Exec(ctx, listQ, 1)
	_ = c.Query(ctx, "SELECT 1")
	_ = c.QueryRow(ctx, insertQ+" RETURNING id")
	_ = c.Exec(ctx, "SELECT set_config('app.request_id', $1, true)", "x")
	b.Queue(listQ)
	return nil
}

// Look-alikes: not SQL, not a connection, not a batch.
func other(r *http.Request, j *jobs, sb *strings.Builder, name string) string {
	_ = r.URL.Query()
	j.Queue(name)
	sb.WriteString("hello ")
	sb.WriteString(name)
	msg := "read from " + name
	msg += " and then " + name
	return msg + strings.Join([]string{"a", name}, ",")
}
