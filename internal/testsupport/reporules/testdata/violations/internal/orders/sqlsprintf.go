package orders

import "fmt"

// SQL built with Sprintf and with concatenation outside internal/platform/db (INV-04, ADR-003).
func byName(name string) string {
	return fmt.Sprintf("SELECT id FROM app.orders WHERE name = '%s'", name)
}

func byID(id string) string {
	return "SELECT " + "id FROM app.orders WHERE id = " + id
}

func concatOnly(x string) string { return "SELECT " + x }

func update(table string) string {
	return "UPDATE " + table + " SET closed = true"
}
