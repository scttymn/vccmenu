package menu

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/web"
)

type userSummary struct {
	ID    int64
	Email string
}
type usersPage struct {
	Users        []userSummary
	CurrentID    int64
	Email, Error string
	Added        bool
}

func (c Controller) usersPage(w http.ResponseWriter, r *http.Request, status int, email, message string) error {
	current, _ := auth.Current(r)
	data := usersPage{CurrentID: current.ID, Email: email, Error: message, Added: r.URL.Query().Get("added") == "1"}
	rows, err := c.DB.Read.QueryContext(r.Context(), "SELECT id, email_address FROM users ORDER BY email_address")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var user userSummary
		if err := rows.Scan(&user.ID, &user.Email); err != nil {
			return err
		}
		data.Users = append(data.Users, user)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	return pages.ExecuteTemplate(w, "users.html", data)
}
func (c Controller) Users(w http.ResponseWriter, r *http.Request) error {
	return c.usersPage(w, r, 200, "", "")
}
func (c Controller) AddUser(w http.ResponseWriter, r *http.Request) error {
	current, ok := auth.Current(r)
	if !ok {
		return web.Status(401, errors.New("Please sign in again."))
	}
	email := auth.Normalize(r.PostFormValue("email"))
	invalid := func(status int, message string) error { return c.usersPage(w, r, status, email, message) }
	if !c.Auth.Limits.Allow("add-user:"+web.IDText(current.ID), 10, 3*time.Minute, time.Now()) {
		return invalid(429, "Please wait a few minutes before adding another user.")
	}
	if address, err := mail.ParseAddress(email); err != nil || address.Address != email || len(email) > 254 {
		return invalid(422, "Enter a valid email address.")
	}
	password := r.PostFormValue("password")
	if problems := auth.DefaultRules(password, r.PostFormValue("confirmation")); len(problems) > 0 {
		return invalid(422, strings.Join(problems, ". "))
	}
	digest, err := auth.Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	result, err := c.DB.Write.ExecContext(r.Context(), `INSERT INTO users (email_address,password_digest,created_at,updated_at) VALUES (?,?,?,?) ON CONFLICT(email_address) DO NOTHING`, email, digest, now, now)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return invalid(409, "A user with that email already exists.")
	}
	http.Redirect(w, r, "/admin/users?added=1", http.StatusSeeOther)
	return nil
}
