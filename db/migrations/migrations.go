// Package migrations embeds the goose SQL migrations so the binary carries its own schema
// (ADR-004). `crm migrate` and the integration harness run them as crm_owner.
package migrations

import "embed"

// FS holds the *.sql files at its root, as goose.NewProvider expects.
//
//go:embed *.sql
var FS embed.FS
