package menu

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scttymn/vccmenu/assets"
)

func TestInitialMenuEscapesScriptContent(t *testing.T) {
	m := Menu{ID: 1, Content: Content{Brand: "</script><script>alert(1)</script>"}}
	w := httptest.NewRecorder()
	if err := page(w, httptest.NewRequest("GET", "/admin/menus/1", nil), 200, editor(editPage{Menu: m})); err != nil {
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
	if !strings.Contains(body, assets.Path("editor.js")) {
		t.Fatal("missing versioned editor asset")
	}
}
