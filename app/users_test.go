package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/vccmenu/app/menu"
)

func TestAnySignedInUserCanAddUsersWithFullAccess(t *testing.T) {
	a := newApp(t)
	h := a.Handler()
	login := auth.Auth{DB: a.DB, Signer: a.Signer}
	user, err := login.CreateUser(t.Context(), "first@example.com", "first-test-password")
	if err != nil {
		t.Fatal(err)
	}
	session := func(u auth.User) *http.Cookie {
		t.Helper()
		w := httptest.NewRecorder()
		if _, err := login.StartSession(w, httptest.NewRequest("GET", "/", nil), u); err != nil {
			t.Fatal(err)
		}
		return w.Result().Cookies()[0]
	}
	first := session(user)
	add := func(email, password, confirmation, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		values := url.Values{"email": {email}, "password": {password}, "confirmation": {confirmation}}
		r := httptest.NewRequest("POST", "/admin/users", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call(h, "GET", "/admin/users", "", nil, ""); w.Code != 302 {
		t.Fatalf("anonymous list: %d", w.Code)
	}
	if w := add("second@example.com", "second-test-password", "second-test-password", "", nil); w.Code != 302 {
		t.Fatalf("anonymous create: %d", w.Code)
	}
	if w := add("second@example.com", "second-test-password", "second-test-password", "https://other.example", first); w.Code != 422 {
		t.Fatalf("cross-origin create: %d", w.Code)
	}
	if w := add("not-an-email", "second-test-password", "second-test-password", "", first); w.Code != 422 {
		t.Fatalf("invalid email: %d", w.Code)
	}
	if w := add("second@example.com", "short", "short", "", first); w.Code != 422 {
		t.Fatalf("weak password: %d", w.Code)
	}
	if w := add("second@example.com", "second-test-password", "mismatched-password", "", first); w.Code != 422 || strings.Contains(w.Body.String(), "second-test-password") {
		t.Fatalf("invalid password response: %d", w.Code)
	}
	if w := add(" SECOND@EXAMPLE.COM ", "second-test-password", "second-test-password", "", first); w.Code != 303 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	secondUser, ok := login.Authenticate(t.Context(), "second@example.com", "second-test-password")
	if !ok {
		t.Fatal("new user cannot sign in")
	}
	second := session(secondUser)
	if w := add("second@example.com", "replacement-password", "replacement-password", "", first); w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if _, ok := login.Authenticate(t.Context(), "second@example.com", "second-test-password"); !ok {
		t.Fatal("duplicate create changed password")
	}
	list := call(h, "GET", "/admin/users", "", second, "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), "first@example.com") || strings.Contains(list.Body.String(), "password_digest") {
		t.Fatal("user list missing or includes private fields")
	}
	if list.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("account list can be cached")
	}
	m, err := menu.Load(t.Context(), a.DB)
	if err != nil {
		t.Fatal(err)
	}
	m.Sections[0].Name = "Coffee"
	body, _ := json.Marshal(m)
	if w := call(h, "PUT", "/admin/menu", string(body), second, ""); w.Code != 200 {
		t.Fatalf("new user cannot edit: %d", w.Code)
	}
	if w := add("third@example.com", "third-test-password", "third-test-password", "", second); w.Code != 303 {
		t.Fatalf("new user cannot add users: %d", w.Code)
	}
	var count int
	if err := a.DB.Read.QueryRowContext(t.Context(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("unexpected users: %d", count)
	}
}
