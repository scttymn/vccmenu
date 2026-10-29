package menu

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/db"
	"github.com/scttymn/gantry/token"
	"github.com/scttymn/gantry/web"
)

type Controller struct {
	DB   *db.DB
	Auth *auth.Auth
}

func page(w http.ResponseWriter, r *http.Request, status int, view templ.Component) error {
	w.Header().Set("Cache-Control", "no-store")
	return web.Render(w, r, status, view)
}
func (c Controller) Display(w http.ResponseWriter, r *http.Request) error {
	return page(w, r, http.StatusOK, display())
}
func (c Controller) Editor(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB)
	if err != nil {
		return err
	}
	return page(w, r, http.StatusOK, editor(m))
}
func (c Controller) Login(w http.ResponseWriter, r *http.Request, p auth.Page) error {
	return page(w, r, http.StatusOK, login(p))
}
func (c Controller) Public(w http.ResponseWriter, r *http.Request) error {
	board, err := Load(r.Context(), c.DB)
	if err != nil {
		return err
	}
	snap := Snapshot{
		Brand:     board.Brand,
		Title:     board.Title,
		ServerNow: time.Now().UTC(),
		Left:      visibleSections(board.AllDay.Sections),
		Timeline:  []Span{},
	}
	return respond(w, snap)
}
func (c Controller) Read(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB)
	if err != nil {
		return err
	}
	return respond(w, m)
}
func respond(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	return json.NewEncoder(w).Encode(v)
}
func (c Controller) Save(w http.ResponseWriter, r *http.Request) error {
	raw, err := readDocument(w, r)
	if err != nil {
		return web.Status(400, errors.New("The menu could not be read."))
	}
	board, err := ParseEditor(raw)
	if err != nil {
		return parseStatus(err)
	}
	if err := board.ensureIDs(); err != nil {
		return err
	}
	if err := board.Validate(); err != nil {
		return web.Status(422, err)
	}
	if err := Save(r.Context(), c.DB, board); err != nil {
		if errors.Is(err, ErrConflict) {
			return web.Status(409, err)
		}
		return err
	}
	board.Revision++
	return respond(w, board)
}

func readDocument(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(w, r.Body, maxDocument))
}

func parseStatus(err error) error {
	var read readError
	if errors.As(err, &read) {
		return web.Status(400, errors.New("The menu could not be read."))
	}
	return web.Status(422, err)
}
func (c Controller) Setup(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Referrer-Policy", "no-referrer")
	return page(w, r, http.StatusOK, setup())
}
func (c Controller) CreateEditor(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	email := auth.Normalize(r.FormValue("email"))
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		return web.Status(422, errors.New("Enter a valid email address."))
	}
	password := r.FormValue("password")
	if errs := auth.DefaultRules(password, r.FormValue("confirmation")); len(errs) > 0 {
		return web.Status(422, errors.New(strings.Join(errs, ". ")))
	}
	// Validate before hashing; consumed atomically with the new user below.
	key := token.Digest(r.FormValue("invitation"))
	var exists int
	if err := c.DB.Read.QueryRowContext(r.Context(), "SELECT count(*) FROM invitations WHERE token_digest = ? AND expires_at > ?", key, time.Now().Unix()).Scan(&exists); err != nil {
		return err
	}
	if exists != 1 {
		return web.Status(403, errors.New("This setup code has expired or was already used."))
	}
	digest, err := auth.Hash(password)
	if err != nil {
		return err
	}
	err = c.DB.Tx(r.Context(), func(tx *db.Tx) error {
		result, err := tx.ExecContext(r.Context(), "DELETE FROM invitations WHERE token_digest = ? AND expires_at > ?", key, time.Now().Unix())
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return web.Status(403, errors.New("This setup code was already used."))
		}
		now := time.Now().UTC()
		_, err = tx.ExecContext(r.Context(), "INSERT INTO users (email_address,password_digest,created_at,updated_at) VALUES (?,?,?,?)", email, digest, now, now)
		return err
	})
	if err != nil {
		return err
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return nil
}

func (c Controller) Password(w http.ResponseWriter, r *http.Request) error {
	return page(w, r, http.StatusOK, password())
}

func (c Controller) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	user, ok := auth.Current(r)
	if !ok {
		return web.Status(401, errors.New("Please sign in again."))
	}
	if !c.Auth.Limits.Allow("change-password:"+web.IDText(user.ID), 10, 3*time.Minute, time.Now()) {
		return web.Status(429, errors.New("Please wait a few minutes before trying again."))
	}
	if _, ok := c.Auth.Authenticate(r.Context(), user.EmailAddress, r.PostFormValue("current_password")); !ok {
		return web.Status(422, errors.New("The current password is incorrect."))
	}
	password := r.PostFormValue("password")
	if problems := auth.DefaultRules(password, r.PostFormValue("confirmation")); len(problems) > 0 {
		return web.Status(422, errors.New(strings.Join(problems, ". ")))
	}
	if err := c.Auth.SetPassword(r.Context(), user.ID, password); err != nil {
		return err
	}
	web.ClearCookie(w, "session")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return nil
}
