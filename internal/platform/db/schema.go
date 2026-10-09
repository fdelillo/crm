package db

import "context"

// SchemaVersion reads the same maximum as goose, using only the column grant to crm_auth.
func SchemaVersion(ctx context.Context, q DBTX) (version int64, ok bool, err error) {
	var value *int64
	if err = q.QueryRow(ctx, "SELECT max(version_id) FROM public.goose_db_version").Scan(&value); err != nil {
		return 0, false, MapError(err)
	}
	if value == nil {
		return 0, false, nil
	}
	return *value, true, nil
}
