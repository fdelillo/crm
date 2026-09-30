// Package sqlcspike is the sqlc canary of the project: its queries run against the REAL schema (all the
// migrations, with their policies, column-level GRANTs and functions with $$ bodies), so `make lint`
// (sqlc diff) fails if sqlc stops parsing the schema and the type overrides of ADR-003 stay
// verified. The first module with real queries (a store/ package) replaces it: delete this package
// and the `spike` block of sqlc.yaml then. Nothing in production imports it.
package sqlcspike
