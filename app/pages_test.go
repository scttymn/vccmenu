package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scttymn/gantry/auth"
	"github.com/scttymn/gantry/testkit"
	"github.com/scttymn/vccmenu/assets"
)

func TestMenuPagesAndAssetLinks(t *testing.T) {
	a := newApp(t)
	h := a.Handler()
	login := auth.Auth{DB: a.DB, Signer: a.Signer}
	u, err := login.CreateUser(t.Context(), "pages@example.com", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	session := httptest.NewRecorder()
	if _, err := login.StartSession(session, httptest.NewRequest("GET", "/", nil), u); err != nil {
		t.Fatal(err)
	}
	cookie := session.Result().Cookies()[0]
	for _, tc := range []struct {
		path, title string
		private     bool
	}{
		{"/", "Valley Coffee Co Menu", false},
		{"/login", "Sign in", false},
		{"/setup", "Set up your account", false},
		{"/admin/", "Edit menu", true},
		{"/admin/password", "Change password", true},
		{"/admin/users", "Users", true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Accept", "text/html")
			if tc.private {
				r.AddCookie(cookie)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<title>"+tc.title) {
				t.Fatalf("page failed: %d %s", w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("page can retain outdated content or account data")
			}
			testkit.Links(t, h, testkit.Page{Path: tc.path, Header: r.Header})
		})
	}
}

func TestMenuAssetsUseGantryCaching(t *testing.T) {
	h := handler(t)
	for _, name := range []string{"display.css", "editor.css", "display.js", "editor.js", "navigation.js", "password.js", "setup.js"} {
		w := get(t, h, assets.Path(name))
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("asset %s: %d %s", name, w.Code, w.Header())
		}
	}
	for _, path := range []string{"/assets/password.js", "/assets/password-00000000.js", "/menu-assets/password.js"} {
		w := get(t, h, path)
		if w.Code != http.StatusNotFound || strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("invalid or obsolete asset URL: %s: %d %s", path, w.Code, w.Header())
		}
	}
}
