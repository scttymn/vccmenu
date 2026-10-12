package seeds

import (
	"context"
	"time"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/db"
)

// Development adds the local editor account only in an explicitly selected
// development environment. Existing passwords are never reset by seeding.
func Development(ctx context.Context, d *db.DB, environment string) error {
	if environment != "development" {
		return nil
	}
	const email = "admin@example.com"
	var exists int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE email_address = ?", email).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	digest, err := auth.Hash("Password1!")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = d.Write.ExecContext(ctx, `INSERT INTO users (email_address,password_digest,created_at,updated_at)
VALUES (?,?,?,?) ON CONFLICT(email_address) DO NOTHING`, email, digest, now, now)
	return err
}
