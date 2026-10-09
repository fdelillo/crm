package migrations_test

import (
	"github.com/fdelillo/crm/db/migrations"
	"testing"
	"testing/fstest"
)

func TestExpectedVersion(t *testing.T) {
	for _, tc := range []struct {
		fs   fstest.MapFS
		want int64
		bad  bool
	}{
		{fstest.MapFS{}, 0, true},
		{fstest.MapFS{"bad.sql": {Data: nil}}, 0, true},
		{fstest.MapFS{"00002_two.sql": {}, "00001_one.sql": {}, "note.txt": {}}, 2, false},
	} {
		v, err := migrations.ExpectedVersion(tc.fs)
		if (err != nil) != tc.bad || v != tc.want {
			t.Errorf("%v version=%d err=%v", tc.fs, v, err)
		}
	}
}
