package testenv

import "testing"

func TestSlugForDir(t *testing.T) {
	cases := []struct {
		dir  string
		want string
	}{
		{"/home/dev/avtotest/backend/internal/billing", "internal_billing"},
		{"/home/dev/avtotest/backend/internal/billing/payme", "internal_billing_payme"},
		// The module root itself has nothing below it.
		{"/home/dev/avtotest/backend", "unknown"},
		// Same leaf under different parents must not collapse into one slug,
		// or the two packages would share a database.
		{"/home/dev/avtotest/backend/cmd/importer", "cmd_importer"},
		{"/home/dev/avtotest/backend/internal/importer", "internal_importer"},
		// A checkout path with characters that are illegal in an SQL
		// identifier: only the part below the module root is kept, so the
		// Cyrillic desktop directory in the dev setup cannot leak in.
		{"/home/sher/Рабочий стол/avtotest/backend/internal/session", "internal_session"},
		// Outside a module root, fall back to sanitizing the whole path rather
		// than silently returning the same slug for everything.
		{"/tmp/scratch-dir", "tmp_scratch_dir"},
	}
	for _, c := range cases {
		if got := SlugForDir(c.dir); got != c.want {
			t.Errorf("SlugForDir(%q) = %q, want %q", c.dir, got, c.want)
		}
	}
}
