package db

// SetFault installs a hook that runs before every SET LOCAL ROLE of transactions started by r; if it
// returns an error, that error is used as if PostgreSQL had refused the SET. Tests only.
func SetFault(r TxRunner, fault func(role string) error) { r.(*runner).fault = fault }
