package menu

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/scttymn/gantry/db"
)

//go:embed initial.json
var Initial []byte

type Item struct {
	Name    string `json:"name"`
	Price   string `json:"price"`
	Visible bool   `json:"visible"`
}
type Section struct {
	Name   string   `json:"name"`
	Note   string   `json:"note"`
	Footer string   `json:"footer"`
	Extras []string `json:"extras"`
	Items  []Item   `json:"items"`
}
type Menu struct {
	Brand    string    `json:"brand"`
	Title    string    `json:"title"`
	Sections []Section `json:"sections"`
	Revision int64     `json:"revision"`
}

func Seed(ctx context.Context, d *db.DB) error {
	_, err := d.Write.ExecContext(ctx, "INSERT INTO menus (id, content) VALUES (1, ?) ON CONFLICT(id) DO NOTHING", string(Initial))
	return err
}
func Load(ctx context.Context, d *db.DB) (Menu, error) {
	var m Menu
	var content string
	var revision int64
	if err := d.Read.QueryRowContext(ctx, "SELECT content, revision FROM menus WHERE id = 1").Scan(&content, &revision); err != nil {
		return m, err
	}
	err := json.Unmarshal([]byte(content), &m)
	m.Revision = revision
	return m, err
}
func (m Menu) Validate() error {
	valid := func(s string, max int, required bool) bool {
		return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && (!required || strings.TrimSpace(s) != "")
	}
	if !valid(m.Brand, 60, true) || !valid(m.Title, 30, true) {
		return errors.New("Enter a shop name (up to 60 characters) and title (up to 30).")
	}
	if len(m.Sections) != 3 {
		return errors.New("The TV menu needs three sections.")
	}
	for _, s := range m.Sections {
		if !valid(s.Name, 30, true) || !valid(s.Note, 120, false) || !valid(s.Footer, 120, false) {
			return errors.New("Check the section title and notes; one is missing or too long.")
		}
		if len(s.Items) > 40 || len(s.Extras) > 20 {
			return errors.New("Each section can have up to 40 items and 20 toppings.")
		}
		for _, i := range s.Items {
			if !valid(i.Name, 80, true) || !valid(i.Price, 40, true) {
				return fmt.Errorf("In %s, each item needs a name (up to 80 characters) and price (up to 40).", s.Name)
			}
		}
		for _, e := range s.Extras {
			if !valid(e, 60, true) {
				return errors.New("Topping names must be 1–60 characters.")
			}
		}
	}
	return nil
}

var ErrConflict = errors.New("Someone else updated the menu. Reload before saving so their changes are kept.")

func Save(ctx context.Context, d *db.DB, m Menu) error {
	if err := m.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	result, err := d.Write.ExecContext(ctx, "UPDATE menus SET content = ?, revision = revision + 1 WHERE id = 1 AND revision = ?", string(b), m.Revision)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
