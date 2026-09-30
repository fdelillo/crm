package main

import (
	"context"
	"errors"
)

// runTenants holds the operational commands on companies. reprovision-roles is a placeholder until
// Phase 9 (T-B906): it needs the provisioning function of Phase 1 and the database pool.
func runTenants(_ context.Context, args []string, _ env) error {
	if len(args) == 0 {
		return usagef("tenants: missing subcommand (reprovision-roles)")
	}
	switch args[0] {
	case "reprovision-roles":
		return errors.New("tenants reprovision-roles: not implemented yet (planned for Phase 9, T-B906)")
	default:
		return usagef("tenants: unknown subcommand %q", args[0])
	}
}
