package migrations

import (
	"fmt"
	"github.com/pressly/goose/v3"
	"io/fs"
	"strings"
)

// ExpectedVersion returns the largest embedded SQL migration number, rejecting malformed names.
func ExpectedVersion(fsys fs.FS) (int64, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return 0, fmt.Errorf("migrations: read: %w", err)
	}
	var maximum int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := goose.NumericComponent(entry.Name())
		if err != nil {
			return 0, fmt.Errorf("migrations: %w", err)
		}
		if version > maximum {
			maximum = version
		}
	}
	if maximum == 0 {
		return 0, fmt.Errorf("migrations: no SQL migration")
	}
	return maximum, nil
}
