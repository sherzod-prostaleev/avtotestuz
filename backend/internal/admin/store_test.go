package admin

import (
	"testing"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/testdb"
)

func TestEnsureSuperadminIdempotent(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	store := Store{Pool: pool}

	id1, err := store.EnsureSuperadmin(t.Context(), "Admin@Example.UZ", "password123", "One")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := store.EnsureSuperadmin(t.Context(), "admin@example.uz", "password456", "Two")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("ids differ %s vs %s", id1, id2)
	}
	u, err := store.GetUserByEmail(t.Context(), "admin@example.uz")
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "Two" {
		t.Fatalf("display_name=%q", u.DisplayName)
	}
	if !auth.CheckPassword(u.PasswordHash, "password456") {
		t.Fatal("password not updated")
	}
	roles, err := store.ListRoles(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(roles, "superadmin") {
		t.Fatalf("roles=%v", roles)
	}
}
