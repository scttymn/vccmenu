package menu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const maxDocument = 256 << 10

type readError struct{ err error }

func (e readError) Error() string {
	if e.err == nil {
		return "The menu could not be read."
	}
	return e.err.Error()
}

func (e readError) Unwrap() error { return e.err }

type limitError struct{ msg string }

func (e limitError) Error() string { return e.msg }

func newLimit(msg string) error { return limitError{msg: msg} }

func ParseStored(raw []byte) (Board, error) {
	keys, err := topKeys(raw)
	if err != nil {
		return Board{}, err
	}
	_, sections := keys["sections"]
	_, allDay := keys["allDay"]
	if sections && allDay {
		return Board{}, readError{err: errors.New("menu has both sections and allDay")}
	}
	if sections {
		return parseLegacy(raw)
	}
	return parseBoard(raw)
}

func ParseEditor(raw []byte) (Board, error) {
	keys, err := topKeys(raw)
	if err != nil {
		return Board{}, err
	}
	if _, ok := keys["sections"]; ok {
		return Board{}, readError{err: errors.New("legacy menu")}
	}
	return parseBoard(raw)
}

func topKeys(raw []byte) (map[string]json.RawMessage, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, readError{err: err}
	}
	return keys, nil
}

func decodeStrict(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var lim limitError
		if errors.As(err, &lim) {
			return err
		}
		var read readError
		if errors.As(err, &read) {
			return err
		}
		return readError{err: err}
	}
	if dec.More() {
		return readError{err: errors.New("trailing data")}
	}
	return nil
}

type legacyDoc struct {
	Brand    string    `json:"brand"`
	Title    string    `json:"title"`
	Sections []Section `json:"sections"`
	Revision int64     `json:"revision"`
}

func parseLegacy(raw []byte) (Board, error) {
	var doc legacyDoc
	if err := decodeStrict(raw, &doc); err != nil {
		return Board{}, err
	}
	body, err := menuFromParts(MenuID("all-day"), "All day", doc.Sections, nil)
	if err != nil {
		return Board{}, err
	}
	zone, err := ParseZone("America/Chicago")
	if err != nil {
		return Board{}, err
	}
	board := Board{
		Brand:    doc.Brand,
		Title:    doc.Title,
		Zone:     zone,
		AllDay:   body,
		Dayparts: []Daypart{},
		Revision: doc.Revision,
	}
	board.normalize()
	return board, nil
}

type boardDoc struct {
	Brand    string       `json:"brand"`
	Title    string       `json:"title"`
	Zone     Zone         `json:"timezone"`
	AllDay   menuDoc      `json:"allDay"`
	Dayparts []daypartDoc `json:"dayparts"`
	Revision int64        `json:"revision"`
}

type menuDoc struct {
	ID       MenuID    `json:"id"`
	Name     string    `json:"name"`
	Sections []Section `json:"sections"`
	Specials []Combo   `json:"specials"`
}

type daypartDoc struct {
	ID       MenuID    `json:"id"`
	Name     string    `json:"name"`
	Sections []Section `json:"sections"`
	Specials []Combo   `json:"specials"`
	When     When      `json:"when"`
}

func parseBoard(raw []byte) (Board, error) {
	var doc boardDoc
	if err := decodeStrict(raw, &doc); err != nil {
		return Board{}, err
	}
	allDay, err := menuFromParts(doc.AllDay.ID, doc.AllDay.Name, doc.AllDay.Sections, doc.AllDay.Specials)
	if err != nil {
		return Board{}, err
	}
	dayparts := make([]Daypart, 0, len(doc.Dayparts))
	for _, part := range doc.Dayparts {
		body, err := menuFromParts(part.ID, part.Name, part.Sections, part.Specials)
		if err != nil {
			return Board{}, err
		}
		dayparts = append(dayparts, Daypart{MenuBody: body, When: part.When})
	}
	board := Board{
		Brand:    doc.Brand,
		Title:    doc.Title,
		Zone:     doc.Zone,
		AllDay:   allDay,
		Dayparts: dayparts,
		Revision: doc.Revision,
	}
	board.normalize()
	return board, nil
}

func menuFromParts(id MenuID, name string, sections []Section, specials []Combo) (MenuBody, error) {
	if len(sections) != 3 {
		return MenuBody{}, readError{err: errors.New("The TV menu needs three sections.")}
	}
	var trio [3]Section
	copy(trio[:], sections)
	return MenuBody{ID: id, Name: name, Sections: trio, Specials: specials}, nil
}

func (z Zone) MarshalJSON() ([]byte, error) {
	if z.name == "" {
		return []byte("null"), nil
	}
	return json.Marshal(z.name)
}

func (z *Zone) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*z = Zone{}
		return nil
	}
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return readError{err: err}
	}
	if name == "" {
		*z = Zone{}
		return nil
	}
	parsed, err := ParseZone(name)
	if err != nil {
		return readError{err: err}
	}
	*z = parsed
	return nil
}

func (w When) MarshalJSON() ([]byte, error) {
	switch w.kind {
	case kindWeekly:
		return json.Marshal(struct {
			Kind  string   `json:"kind"`
			Days  []string `json:"days"`
			Start string   `json:"start"`
			End   string   `json:"end"`
		}{Kind: "weekly", Days: w.weekly.dayNames(), Start: w.weekly.start.String(), End: w.weekly.end.String()})
	case kindOneOff:
		return json.Marshal(struct {
			Kind  string `json:"kind"`
			First string `json:"first"`
			Last  string `json:"last"`
			From  string `json:"from"`
			To    string `json:"to"`
		}{Kind: "oneOff", First: w.once.first.String(), Last: w.once.last.String(), From: w.once.from.String(), To: w.once.to.String()})
	default:
		return nil, errors.New("menu window is missing")
	}
}

func (w *When) UnmarshalJSON(raw []byte) error {
	if string(bytes.TrimSpace(raw)) == "null" {
		return readError{err: errors.New("window is null")}
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return readError{err: err}
	}
	kindRaw, ok := keys["kind"]
	if !ok {
		return readError{err: errors.New("window kind is missing")}
	}
	var kind string
	if err := json.Unmarshal(kindRaw, &kind); err != nil {
		return readError{err: err}
	}
	switch kind {
	case "weekly":
		return w.unmarshalWeekly(keys)
	case "oneOff":
		return w.unmarshalOneOff(keys)
	default:
		return readError{err: fmt.Errorf("unknown window kind %q", kind)}
	}
}

func (w *When) unmarshalWeekly(keys map[string]json.RawMessage) error {
	if err := allowKeys(keys, "kind", "days", "start", "end"); err != nil {
		return err
	}
	for _, key := range []string{"days", "start", "end"} {
		if _, ok := keys[key]; !ok {
			return readError{err: fmt.Errorf("weekly window needs %s", key)}
		}
	}
	var days []string
	if err := json.Unmarshal(keys["days"], &days); err != nil {
		return readError{err: err}
	}
	var startText, endText string
	if err := json.Unmarshal(keys["start"], &startText); err != nil {
		return readError{err: err}
	}
	if err := json.Unmarshal(keys["end"], &endText); err != nil {
		return readError{err: err}
	}
	start, err := ParseClock(startText)
	if err != nil {
		return readError{err: err}
	}
	end, err := ParseClock(endText)
	if err != nil {
		return readError{err: err}
	}
	weekdays := make([]time.Weekday, 0, len(days))
	seen := map[string]bool{}
	for _, day := range days {
		weekday, ok := weekdayName[day]
		if !ok || seen[day] {
			return readError{err: fmt.Errorf("bad day %q", day)}
		}
		seen[day] = true
		weekdays = append(weekdays, weekday)
	}
	parsed, err := Weekly(weekdays, start, end)
	if err != nil {
		return err
	}
	*w = parsed
	return nil
}

func (w *When) unmarshalOneOff(keys map[string]json.RawMessage) error {
	if err := allowKeys(keys, "kind", "first", "last", "from", "to"); err != nil {
		return err
	}
	for _, key := range []string{"first", "last"} {
		if _, ok := keys[key]; !ok {
			return readError{err: fmt.Errorf("one-off window needs %s", key)}
		}
	}
	var firstText, lastText string
	if err := json.Unmarshal(keys["first"], &firstText); err != nil {
		return readError{err: err}
	}
	if err := json.Unmarshal(keys["last"], &lastText); err != nil {
		return readError{err: err}
	}
	first, err := ParseDate(firstText)
	if err != nil {
		return readError{err: err}
	}
	last, err := ParseDate(lastText)
	if err != nil {
		return readError{err: err}
	}
	fromText, err := optionalClock(keys, "from", "00:00")
	if err != nil {
		return err
	}
	toText, err := optionalClock(keys, "to", "24:00")
	if err != nil {
		return err
	}
	from, err := ParseClock(fromText)
	if err != nil {
		return readError{err: err}
	}
	to, err := ParseClock(toText)
	if err != nil {
		return readError{err: err}
	}
	parsed, err := OneOff(first, last, from, to)
	if err != nil {
		return err
	}
	*w = parsed
	return nil
}

func optionalClock(keys map[string]json.RawMessage, key, fallback string) (string, error) {
	raw, ok := keys[key]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return fallback, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", readError{err: err}
	}
	if text == "" {
		return fallback, nil
	}
	return text, nil
}

func allowKeys(keys map[string]json.RawMessage, allowed ...string) error {
	ok := map[string]bool{}
	for _, key := range allowed {
		ok[key] = true
	}
	for key := range keys {
		if !ok[key] {
			return readError{err: fmt.Errorf("unknown field %q", key)}
		}
	}
	return nil
}
