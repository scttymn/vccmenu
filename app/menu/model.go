package menu

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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

type MenuID string

type Board struct {
	Brand    string    `json:"brand"`
	Title    string    `json:"title"`
	Zone     Zone      `json:"timezone"`
	AllDay   MenuBody  `json:"allDay"`
	Dayparts []Daypart `json:"dayparts"`
	Revision int64     `json:"revision"`
}

type MenuBody struct {
	ID       MenuID     `json:"id"`
	Name     string     `json:"name"`
	Sections [3]Section `json:"sections"`
	Specials []Combo    `json:"specials"`
}

type Daypart struct {
	MenuBody
	When When `json:"when"`
}

type Combo struct {
	Number     int    `json:"number"`
	Name       string `json:"name"`
	Contents   string `json:"contents"`
	Price      string `json:"price"`
	Individual string `json:"individual,omitempty"`
	Savings    string `json:"savings,omitempty"`
}

var menuIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,31}$`)

func Seed(ctx context.Context, d *db.DB) error {
	_, err := d.Write.ExecContext(ctx, "INSERT INTO menus (id, content) VALUES (1, ?) ON CONFLICT(id) DO NOTHING", string(Initial))
	return err
}

func Load(ctx context.Context, d *db.DB) (Board, error) {
	var content string
	var revision int64
	if err := d.Read.QueryRowContext(ctx, "SELECT content, revision FROM menus WHERE id = 1").Scan(&content, &revision); err != nil {
		return Board{}, err
	}
	board, err := ParseStored([]byte(content))
	if err != nil {
		return Board{}, err
	}
	board.Revision = revision
	return board, nil
}

func (b Board) Validate() error {
	valid := func(s string, max int, required bool) bool {
		return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && (!required || strings.TrimSpace(s) != "")
	}
	if b.Zone.Location() == nil || b.Zone.Name() == "" {
		return errors.New("Choose a timezone.")
	}
	if !valid(b.Brand, 60, true) || !valid(b.Title, 30, true) {
		return errors.New("Enter a shop name (up to 60 characters) and title (up to 30).")
	}
	if len(b.Dayparts) > 16 {
		return errors.New("You can schedule up to 16 menus.")
	}
	seen := map[MenuID]bool{}
	if err := validateMenu(b.AllDay, true, valid, seen); err != nil {
		return err
	}
	for _, daypart := range b.Dayparts {
		if daypart.When.IsZero() {
			return errors.New("Each scheduled menu needs hours or a date range.")
		}
		if err := validateMenu(daypart.MenuBody, false, valid, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateMenu(body MenuBody, allDay bool, valid func(string, int, bool) bool, seen map[MenuID]bool) error {
	if !validMenuID(body.ID) || seen[body.ID] {
		return errors.New("Each menu needs its own id.")
	}
	seen[body.ID] = true
	if !valid(body.Name, 40, true) {
		return errors.New("Each menu needs a name (up to 40 characters).")
	}
	for _, section := range body.Sections {
		nameRequired := allDay || sectionHasContent(section)
		if !valid(section.Name, 30, nameRequired) || !valid(section.Note, 120, false) || !valid(section.Footer, 120, false) {
			return errors.New("Check the section title and notes; one is missing or too long.")
		}
		if len(section.Items) > 40 || len(section.Extras) > 20 {
			return errors.New("Each section can have up to 40 items and 20 toppings.")
		}
		for _, item := range section.Items {
			if !valid(item.Name, 80, true) || !valid(item.Price, 40, true) {
				return fmt.Errorf("In %s, each item needs a name (up to 80 characters) and price (up to 40).", section.Name)
			}
		}
		for _, extra := range section.Extras {
			if !valid(extra, 60, true) {
				return errors.New("Topping names must be 1–60 characters.")
			}
		}
	}
	return validateCombos(body.Specials, valid)
}

func sectionHasContent(section Section) bool {
	if strings.TrimSpace(section.Note) != "" || strings.TrimSpace(section.Footer) != "" {
		return true
	}
	return len(section.Items) > 0 || len(section.Extras) > 0
}

func validateCombos(specials []Combo, valid func(string, int, bool) bool) error {
	if len(specials) > 20 {
		return errors.New("Each menu can have up to 20 specials.")
	}
	seen := map[int]bool{}
	for _, combo := range specials {
		if combo.Number < 1 || combo.Number > 99 || seen[combo.Number] {
			return errors.New("Combo numbers must be from 1 to 99, and each menu can use a number only once.")
		}
		seen[combo.Number] = true
		if !valid(combo.Name, 80, true) || !valid(combo.Price, 40, true) {
			return errors.New("Each special needs a name (up to 80 characters) and a price (up to 40).")
		}
		if !valid(combo.Contents, 160, true) {
			return errors.New("Each special needs a description (up to 160 characters).")
		}
		if !valid(combo.Individual, 40, false) || !valid(combo.Savings, 40, false) {
			return errors.New("Individual total and savings must be 40 characters or fewer.")
		}
	}
	return nil
}

func validMenuID(id MenuID) bool {
	return menuIDPattern.MatchString(string(id))
}

func (b *Board) ensureIDs() error {
	seen := map[MenuID]bool{}
	if b.AllDay.ID != "" {
		seen[b.AllDay.ID] = true
	}
	for i := range b.Dayparts {
		if b.Dayparts[i].ID != "" {
			continue
		}
		var id MenuID
		var err error
		for try := 0; try < 5; try++ {
			id, err = NewMenuID()
			if err != nil {
				return err
			}
			if !seen[id] {
				break
			}
		}
		b.Dayparts[i].ID = id
		seen[id] = true
	}
	return nil
}

func (b *Board) normalize() {
	b.AllDay.normalize()
	if b.Dayparts == nil {
		b.Dayparts = []Daypart{}
	}
	for i := range b.Dayparts {
		b.Dayparts[i].MenuBody.normalize()
	}
}

func (m *MenuBody) normalize() {
	if m.Specials == nil {
		m.Specials = []Combo{}
	}
	for i := range m.Sections {
		if m.Sections[i].Items == nil {
			m.Sections[i].Items = []Item{}
		}
		if m.Sections[i].Extras == nil {
			m.Sections[i].Extras = []string{}
		}
	}
}

var ErrConflict = errors.New("Someone else updated the menu. Reload before saving so their changes are kept.")

func Save(ctx context.Context, d *db.DB, b Board) error {
	if err := b.ensureIDs(); err != nil {
		return err
	}
	b.normalize()
	if err := b.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	result, err := d.Write.ExecContext(ctx, "UPDATE menus SET content = ?, revision = revision + 1 WHERE id = 1 AND revision = ?", string(raw), b.Revision)
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
