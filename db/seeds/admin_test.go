package seeds_test

import (
	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/vccmenu/db/seeds"
	"github.com/scttymn/vccmenu/test"
	"testing"
)

func TestAdminBootstrapDoesNotResetOrAddAccounts(t *testing.T) {
	d := test.DB(t)
	if err := seeds.EnsureAdmin(t.Context(), d, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := seeds.EnsureAdmin(t.Context(), d, "admin@example.com", "weak"); err == nil {
		t.Fatal("accepted a weak password")
	}
	if err := seeds.EnsureAdmin(t.Context(), d, "admin@example.com", "test-admin-password"); err != nil {
		t.Fatal(err)
	}
	a := auth.Auth{DB: d}
	if _, ok := a.Authenticate(t.Context(), "admin@example.com", "test-admin-password"); !ok {
		t.Fatal("admin cannot sign in")
	}
	if err := seeds.EnsureAdmin(t.Context(), d, "admin@example.com", "different-password"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Authenticate(t.Context(), "admin@example.com", "test-admin-password"); !ok {
		t.Fatal("existing password was reset")
	}
	if err := seeds.EnsureAdmin(t.Context(), d, "another@example.com", "test-other-password"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := d.Read.QueryRowContext(t.Context(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("bootstrap made %d users", count)
	}
}
