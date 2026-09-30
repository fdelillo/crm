package db

import "fmt"

// internal/platform/db is where the role switch and dynamic SQL are allowed.
func setRole(role string) string { return fmt.Sprintf("SET LOCAL ROLE %s", role) }

func reset() string { return "RESET ROLE" }

func build(cols string) string { return "SELECT " + cols + " FROM app.things" }
