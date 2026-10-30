package menu

import (
	"encoding/json"
	"errors"
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
	Zone *time.Location // the shop's, which schedules are in
}

func (c Controller) zone() *time.Location {
	if c.Zone == nil {
		return time.UTC
	}
	return c.Zone
}

func page(w http.ResponseWriter, r *http.Request, status int, view templ.Component) error {
	w.Header().Set("Cache-Control", "no-store")
	return web.Render(w, r, status, view)
}
func (c Controller) Display(w http.ResponseWriter, r *http.Request) error {
	return page(w, r, http.StatusOK, display(displayPage{Endpoint: "/api/menu"}))
}

// Preview is the menu page for one menu, live or not: its saved version, or
// the editor's unsaved draft while one is open.
func (c Controller) Preview(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB, web.ID(r, "id"))
	if err != nil {
		return err
	}
	id := web.IDText(m.ID)
	return page(w, r, http.StatusOK, display(displayPage{Endpoint: "/admin/menus/" + id + "/display", Draft: "vcc-preview:" + id, Name: m.Name}))
}

// PreviewData is a menu's saved version, as it would show.
func (c Controller) PreviewData(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB, web.ID(r, "id"))
	if err != nil {
		return err
	}
	return respond(w, shown(m))
}
func (c Controller) Login(w http.ResponseWriter, r *http.Request, p auth.Page) error {
	return page(w, r, http.StatusOK, login(p))
}

// shown is m as the menu page draws it: its heading, and only the items
// on the menu.
func shown(m Menu) any {
	sections := make([]Section, len(m.Sections))
	for k, s := range m.Sections {
		s.Items = make([]Item, 0)
		for _, item := range m.Sections[k].Items {
			if item.Visible {
				s.Items = append(s.Items, item)
			}
		}
		sections[k] = s
	}
	return struct {
		Content
		Revision int64 `json:"revision"`
	}{Content{Brand: m.Brand, Title: m.Heading(), Sections: sections}, m.Revision}
}

// Public is the menu live now, as the menu page draws it.
func (c Controller) Public(w http.ResponseWriter, r *http.Request) error {
	menus, err := All(r.Context(), c.DB)
	if err != nil {
		return err
	}
	m, ok := NewTimeline(menus, time.Now(), c.zone()).At(time.Now())
	if !ok {
		return web.Status(404, errors.New("No menu is published."))
	}
	return respond(w, shown(m))
}
func respond(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	return json.NewEncoder(w).Encode(v)
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
