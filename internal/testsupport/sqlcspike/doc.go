// Package sqlcspike is the spike of T-B008: it proves sqlc parses a schema with CREATE POLICY,
// FORCE ROW LEVEL SECURITY and column-level GRANT, and that the type overrides of ADR-003 work.
// The code in this package is generated from schema.sql and queries.sql; nothing in production
// imports it. Delete it once the real Phase 1 modules exercise the same syntax.
package sqlcspike
