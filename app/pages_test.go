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

func TestEditorUsesClockInputs(t *testing.T) {
	a := newApp(t)
	h := a.Handler()
	script := get(t, h, assets.Path("editor.js"))
	body := script.Body.String()
	if script.Code != http.StatusOK || !strings.Contains(body, "type = 'time'") || !strings.Contains(body, "type = 'date'") || strings.Contains(body, "datetime-local") {
		t.Fatalf("editor inputs: %d", script.Code)
	}
	if !strings.Contains(body, "/admin/menu/copy") || !strings.Contains(body, "Always on the left") {
		t.Fatal("editor cannot add or copy a scheduled menu")
	}
	login := auth.Auth{DB: a.DB, Signer: a.Signer}
	user, err := login.CreateUser(t.Context(), "editor-page@example.com", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	session := httptest.NewRecorder()
	if _, err := login.StartSession(session, httptest.NewRequest("GET", "/", nil), user); err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRequest("GET", "/admin/", nil)
	page.Header.Set("Accept", "text/html")
	page.AddCookie(session.Result().Cookies()[0])
	w := httptest.NewRecorder()
	h.ServeHTTP(w, page)
	html := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(html, `id="new-menu"`) || !strings.Contains(html, `id="copy-menu"`) || !strings.Contains(html, `id="timezone"`) || strings.Contains(html, "datetime-local") {
		t.Fatalf("editor page: %d", w.Code)
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
