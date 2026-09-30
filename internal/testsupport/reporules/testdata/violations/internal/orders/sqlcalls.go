package orders

import (
	"context"
	"fmt"
	"strings"
)

type conn interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) error
}

type batch struct{}

func (b *batch) Queue(sql string, args ...any) {}

func viaVariable(ctx context.Context, c conn, q string) error {
	return c.Exec(ctx, q)
}

func viaBuilder(ctx context.Context, c conn, table string) error {
	var sb strings.Builder
	sb.WriteString("SELECT id FROM ")
	sb.WriteString(table)
	return c.Query(ctx, sb.String())
}

func viaAppend(ctx context.Context, c conn, name string) error {
	q := "SELECT id FROM app.t WHERE 1=1"
	q += " AND name = '" + name + "'"
	return c.QueryRow(ctx, q)
}

func viaJoin(t string) string {
	return strings.Join([]string{"DELETE FROM", t}, " ")
}

func viaSprintfWithLeadingNoise(name string) string {
	return fmt.Sprintf("  /* c */ SELECT id FROM app.t WHERE n = '%s'", name)
}

func viaBatch(batchQueue *batch, q string) {
	batchQueue.Queue(q)
}

func viaReplace(t string) string {
	return strings.ReplaceAll("SELECT id FROM TABLE_NAME", "TABLE_NAME", t)
}

func viaFprintf(sb *strings.Builder, t string) {
	fmt.Fprintf(sb, "INSERT INTO %s (id) VALUES ($1)", t)
}

const setConfigWithParameterName = "SELECT set_config($1, $2, true)"

const setConfigSessionAuth = "SELECT set_config('session_authorization', $1, true)"
