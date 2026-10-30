package menu

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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

// Content is what the menu page draws: the shop's name, the heading and one
// to three sections. A blank Title shows the menu's name instead.
type Content struct {
	Brand    string    `json:"brand"`
	Title    string    `json:"title"`
	Sections []Section `json:"sections"`
}

// Menu is one of the shop's menus: its content, the name editors know it
// by, and when it shows.
type Menu struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Content
	Schedule  Schedule `json:"schedule"`
	Published bool     `json:"published"`
	Revision  int64    `json:"revision"`
}

// Heading is the title at the top of the menu page.
func (m Menu) Heading() string {
	if strings.TrimSpace(m.Title) != "" {
		return m.Title
	}
	return m.Name
}

// Items is how many items the menu page shows.
func (m Menu) Items() int {
	n := 0
	for _, s := range m.Sections {
		for _, i := range s.Items {
			if i.Visible {
				n++
			}
		}
	}
	return n
}

// Seed adds the original menu to an empty database, published and not yet
// scheduled, so it's the live menu.
func Seed(ctx context.Context, d *db.DB) error {
	now := time.Now().UTC()
	_, err := d.Write.ExecContext(ctx, `INSERT INTO menus (name, content, schedule, published, created_at, updated_at)
SELECT 'Regular Menu', ?, '{"kind":"weekly","hours":[]}', 1, ?, ? WHERE NOT EXISTS (SELECT 1 FROM menus)`, string(Initial), now, now)
	return err
}

const columns = "id, name, content, schedule, published, revision"

func scan(row interface{ Scan(...any) error }) (Menu, error) {
	var m Menu
	var content, schedule string
	if err := row.Scan(&m.ID, &m.Name, &content, &schedule, &m.Published, &m.Revision); err != nil {
		return m, err
	}
	if err := json.Unmarshal([]byte(content), &m.Content); err != nil {
		return m, err
	}
	return m, json.Unmarshal([]byte(schedule), &m.Schedule)
}
func Load(ctx context.Context, d *db.DB, id int64) (Menu, error) {
	return scan(d.Read.QueryRowContext(ctx, "SELECT "+columns+" FROM menus WHERE id = ?", id))
}

// All is every menu, oldest first.
func All(ctx context.Context, d *db.DB) ([]Menu, error) {
	rows, err := d.Read.QueryContext(ctx, "SELECT "+columns+" FROM menus ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var menus []Menu
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, err
		}
		menus = append(menus, m)
	}
	return menus, rows.Err()
}

func valid(s string, max int, required bool) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && (!required || strings.TrimSpace(s) != "")
}
func (m Menu) Validate() error {
	if !valid(m.Name, 60, true) {
		return errors.New("Give the menu a name of up to 60 characters.")
	}
	if !valid(m.Brand, 60, true) || !valid(m.Title, 30, false) {
		return errors.New("Keep the display title to 30 characters.")
	}
	if len(m.Sections) < 1 || len(m.Sections) > 3 {
		return errors.New("A menu has one to three sections.")
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
	return m.Schedule.Validate()
}

var ErrConflict = errors.New("Someone else updated the menu. Reload before saving so their changes are kept.")
var ErrLastMenu = errors.New("Keep at least one menu, so there’s always something to show.")

func encode(m Menu) (content, schedule string, err error) {
	c, err := json.Marshal(m.Content)
	if err != nil {
		return "", "", err
	}
	s, err := json.Marshal(m.Schedule)
	return string(c), string(s), err
}

func Save(ctx context.Context, d *db.DB, m Menu) error {
	if err := m.Validate(); err != nil {
		return err
	}
	content, schedule, err := encode(m)
	if err != nil {
		return err
	}
	result, err := d.Write.ExecContext(ctx, "UPDATE menus SET name = ?, content = ?, schedule = ?, published = ?, revision = revision + 1, updated_at = ? WHERE id = ? AND revision = ?",
		m.Name, content, schedule, m.Published, time.Now().UTC(), m.ID, m.Revision)
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

// Create adds m as a new menu, and is its id.
func Create(ctx context.Context, d *db.DB, m Menu) (int64, error) {
	if err := m.Validate(); err != nil {
		return 0, err
	}
	content, schedule, err := encode(m)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	result, err := d.Write.ExecContext(ctx, "INSERT INTO menus (name, content, schedule, published, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		m.Name, content, schedule, m.Published, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Delete removes a menu, unless it's the last one.
func Delete(ctx context.Context, d *db.DB, id int64) error {
	return d.Tx(ctx, func(tx *db.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM menus").Scan(&count); err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastMenu
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM menus WHERE id = ?", id)
		return err
	})
}
