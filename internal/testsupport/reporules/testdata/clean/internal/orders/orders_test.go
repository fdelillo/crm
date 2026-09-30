package orders

// Test files are not scanned.
const q = "SET ROLE crm_owner"

func build(t string) string { return "SELECT * FROM " + t }
