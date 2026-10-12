package seeds_test

import (
	"testing"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/vccmenu/db/seeds"
	"github.com/scttymn/vccmenu/test"
)

func TestDevelopmentSeedRequiresExplicitEnvironment(t *testing.T) {
	for _, environment := range []string{"", "production", "test", "staging"} {
		// A non-development run must return before even touching the database.
		if err := seeds.Development(t.Context(), nil, environment); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDevelopmentUserIsRepeatableAndPreservesPasswords(t *testing.T) {
	d := test.DB(t)
	a := auth.Auth{DB: d}
	if _, err := a.CreateUser(t.Context(), "existing@example.com", "existing-password"); err != nil {
		t.Fatal(err)
	}
	if err := seeds.Development(t.Context(), d, "development"); err != nil {
		t.Fatal(err)
	}
	u, ok := a.Authenticate(t.Context(), "admin@example.com", "Password1!")
	if !ok {
		t.Fatal("development user cannot sign in")
	}
	if err := a.SetPassword(t.Context(), u.ID, "changed-password"); err != nil {
		t.Fatal(err)
	}
	if err := seeds.Development(t.Context(), d, "development"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Authenticate(t.Context(), "admin@example.com", "changed-password"); !ok {
		t.Fatal("seeding reset the password")
	}
	if _, ok := a.Authenticate(t.Context(), "existing@example.com", "existing-password"); !ok {
		t.Fatal("seeding changed another user")
	}
	var count int
	if err := d.Read.QueryRowContext(t.Context(), "SELECT count(*) FROM users").Scan(&count); err != nil || count != 2 {
		t.Fatalf("users = %d, error = %v", count, err)
	}
}
