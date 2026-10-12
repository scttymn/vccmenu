package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/token"
	"github.com/scttymn/vccmenu/app/menu"
	"github.com/scttymn/vccmenu/db/seeds"
)

func call(h http.Handler, method, path, body string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestEditingAndPersistence(t *testing.T) {
	a := newApp(t)
	h := a.Handler()
	for _, method := range []string{"GET", "PUT"} {
		w := call(h, method, "/admin/menu", "{}", nil, "")
		if w.Code != 302 {
			t.Fatalf("anonymous %s: %d", method, w.Code)
		}
	}
	login := auth.Auth{DB: a.DB, Signer: a.Signer}
	u, err := login.CreateUser(t.Context(), "editor@example.com", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	_, err = login.StartSession(w, httptest.NewRequest("GET", "/", nil), u)
	if err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	m, err := menu.Load(t.Context(), a.DB)
	if err != nil {
		t.Fatal(err)
	}
	initialRevision := m.Revision
	m.Sections[0].Name = "Coffee & Tea"
	m.Sections[0].Items[0].Price = "$9"
	m.Sections[0].Items[1].Visible = false
	b, _ := json.Marshal(m)
	w = call(h, "PUT", "/admin/menu", string(b), cookie, "https://evil.example")
	if w.Code != 422 {
		t.Fatalf("cross-origin write: %d", w.Code)
	}
	w = call(h, "PUT", "/admin/menu", string(b), cookie, "")
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}
	w = call(h, "PUT", "/admin/menu", string(b), cookie, "")
	if w.Code != 409 {
		t.Fatalf("stale write: %d %s", w.Code, w.Body)
	}
	if err := seeds.Run(t.Context(), a.DB); err != nil {
		t.Fatal(err)
	}
	saved, err := menu.Load(t.Context(), a.DB)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Sections[0].Items[0].Price != "$9" || saved.Revision != initialRevision+1 {
		t.Fatal("seed replaced saved edits")
	}
	pub := call(h, "GET", "/api/menu", "", nil, "")
	if pub.Code != 200 || bytes.Contains(pub.Body.Bytes(), []byte("Americano")) {
		t.Fatalf("hidden item leaked: %d %s", pub.Code, pub.Body)
	}
	if !strings.Contains(pub.Body.String(), "Coffee ") {
		t.Fatal("renamed section missing from TV data")
	}
	if pub.Header().Get("Access-Control-Allow-Origin") != "https://scttymn.github.io" {
		t.Fatal("missing TV origin")
	}
	saved.Sections[0].Items[0].Name = ""
	bad, _ := json.Marshal(saved)
	if w := call(h, "PUT", "/admin/menu", string(bad), cookie, ""); w.Code != 422 {
		t.Fatalf("invalid save: %d", w.Code)
	}
	after, _ := menu.Load(t.Context(), a.DB)
	if after.Revision != saved.Revision {
		t.Fatal("invalid input wrote data")
	}
}
func TestInvitationIsRequiredAndSingleUse(t *testing.T) {
	a := newApp(t)
	h := a.Handler()
	key, digest := token.New("")
	_, err := a.DB.Write.ExecContext(t.Context(), "INSERT INTO invitations (token_digest, expires_at) VALUES (?, ?)", digest, time.Now().Add(time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}
	submit := func(code, email string) *httptest.ResponseRecorder {
		values := url.Values{"invitation": {code}, "email": {email}, "password": {"test-password-123"}, "confirmation": {"test-password-123"}}
		r := httptest.NewRequest("POST", "/setup", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := submit("wrong", "editor@example.com"); w.Code != 403 {
		t.Fatalf("bad invite: %d %s", w.Code, w.Body)
	}
	if w := submit(key, "editor@example.com"); w.Code != 303 {
		t.Fatalf("valid invite: %d %s", w.Code, w.Body)
	}
	if w := submit(key, "another@example.com"); w.Code != 403 {
		t.Fatalf("reused invite: %d", w.Code)
	}
	login := auth.Auth{DB: a.DB, Signer: a.Signer}
	if _, ok := login.Authenticate(t.Context(), "editor@example.com", "test-password-123"); !ok {
		t.Fatal("new editor cannot sign in")
	}
}
