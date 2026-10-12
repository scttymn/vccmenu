package menu

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetCaching(t *testing.T) {
	for _, tc := range []struct{ url, cache string }{
		{assetURL("password.js"), "public, max-age=31536000, immutable"},
		{"/menu-assets/password.js", "no-cache"},
		{"/menu-assets/password.js?v=old", "no-cache"},
	} {
		w := httptest.NewRecorder()
		Static().ServeHTTP(w, httptest.NewRequest("GET", tc.url, nil))
		if w.Code != 200 || w.Header().Get("Cache-Control") != tc.cache {
			t.Fatalf("%s: %d %s", tc.url, w.Code, w.Header())
		}
	}
}

func TestInitialMenuEscapesScriptContent(t *testing.T) {
	m := Menu{Brand: "</script><script>alert(1)</script>"}
	w := httptest.NewRecorder()
	if err := page(w, "editor.html", m); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	marker := `<script id="initial-menu" type="application/json">`
	_, data, ok := strings.Cut(body, marker)
	if !ok {
		t.Fatal("missing initial data")
	}
	data, _, _ = strings.Cut(data, "</script>")
	var got Menu
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatal(err)
	}
	if got.Brand != m.Brand || strings.Contains(data, "<script>") {
		t.Fatal("unsafe or altered menu data")
	}
	if !strings.Contains(body, assetURL("editor.js")) {
		t.Fatal("missing versioned editor asset")
	}
}
