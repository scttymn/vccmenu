package seeds

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/db"
)

// EnsureAdmin bootstraps the first editor from deployment secrets. Existing
// accounts and passwords are never changed, including on later deployments.
func EnsureAdmin(ctx context.Context, d *db.DB, email, password string) error {
	if email == "" && password == "" {
		return nil
	}
	var count int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	email = auth.Normalize(email)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return errors.New("ADMIN_EMAIL must be a valid email address")
	}
	if problems := auth.DefaultRules(password, password); len(problems) > 0 {
		return errors.New("ADMIN_PASSWORD: " + strings.Join(problems, ". "))
	}
	digest, err := auth.Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	// The conditional insert also protects simultaneous first starts.
	_, err = d.Write.ExecContext(ctx, `INSERT INTO users (email_address,password_digest,created_at,updated_at)
 SELECT ?,?,?,? WHERE NOT EXISTS (SELECT 1 FROM users)`, email, digest, now, now)
	return err
}
