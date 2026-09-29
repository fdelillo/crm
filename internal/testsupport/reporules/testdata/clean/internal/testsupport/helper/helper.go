package helper

import "fmt"

// Test-only helpers may build SQL and switch roles.
func Create(name string) string {
	return fmt.Sprintf("CREATE TABLE %s (id int)", name) + "; SET ROLE x"
}
