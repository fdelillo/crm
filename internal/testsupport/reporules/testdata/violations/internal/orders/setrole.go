package orders

// SET ROLE outside internal/platform/db (INV-03).
const switchRole = "SET ROLE crm_t_0123456789abcdef0123456789abcdef"

const switchLocal = `set local role crm_auth`

const undo = "RESET ROLE"

const viaFunction = "SELECT set_config('role', 'crm_owner', false)"
