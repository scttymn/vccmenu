package menu

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/db"
	"github.com/scttymn/gantry/token"
	"github.com/scttymn/gantry/web"
)

//go:embed ui/*
var ui embed.FS
var pages = template.Must(template.New("").Funcs(template.FuncMap{"asset": assetURL}).ParseFS(ui, "ui/*.html"))

func assetURL(name string) string {
	content, err := ui.ReadFile("ui/" + name)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("/menu-assets/%s?v=%x", name, sha256.Sum256(content))
}

type Controller struct {
	DB   *db.DB
	Auth *auth.Auth
}

func Static() http.Handler {
	sub, _ := fs.Sub(ui, "ui")
	files := http.StripPrefix("/menu-assets/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Unversioned TV clients must always revalidate after a deployment.
		w.Header().Set("Cache-Control", "no-cache")
		name := strings.TrimPrefix(r.URL.Path, "/menu-assets/")
		if content, err := ui.ReadFile("ui/" + name); err == nil && r.URL.Query().Get("v") == fmt.Sprintf("%x", sha256.Sum256(content)) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
func page(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	return pages.ExecuteTemplate(w, name, data)
}
func (c Controller) Display(w http.ResponseWriter, r *http.Request) error {
	return page(w, "display.html", nil)
}
func (c Controller) Editor(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB)
	if err != nil {
		return err
	}
	// html/template safely JSON-encodes this value in the script context.
	return page(w, "editor.html", m)
}
func (c Controller) Login(w http.ResponseWriter, r *http.Request, p auth.Page) error {
	return page(w, "login.html", p)
}
func (c Controller) Public(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB)
	if err != nil {
		return err
	}
	for k := range m.Sections {
		visible := make([]Item, 0)
		for _, item := range m.Sections[k].Items {
			if item.Visible {
				visible = append(visible, item)
			}
		}
		m.Sections[k].Items = visible
	}
	// The existing GitHub Pages TV URL reads only this public endpoint.
	w.Header().Set("Access-Control-Allow-Origin", "https://scttymn.github.io")
	return respond(w, m)
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
	var m Menu
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return web.Status(400, errors.New("The menu could not be read."))
	}
	if err := m.Validate(); err != nil {
		return web.Status(422, err)
	}
	if err := Save(r.Context(), c.DB, m); err != nil {
		if errors.Is(err, ErrConflict) {
			return web.Status(409, err)
		}
		return err
	}
	m.Revision++
	return respond(w, m)
}
func (c Controller) Setup(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Referrer-Policy", "no-referrer")
	return page(w, "setup.html", nil)
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
	return page(w, "password.html", nil)
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
