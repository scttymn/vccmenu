package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/scttymn/gantry/web"
)

type menuCard struct {
	ID                 int64
	Number, Name, Line string
	Status             string
	Live, Draft, Ended bool
}
type menusPage struct {
	Live, Until              string // the live menu's name, and when it changes
	Count                    string
	Recurring, Events, Ended []menuCard
	Deleted                  bool
}

// moment is a time near now in words: "2:00 PM" today, "Wed 5:00 PM" this
// week, else "Nov 14, 2:00 PM".
func moment(t, now time.Time) string {
	days := calendarDays(now, t)
	switch {
	case days == 0:
		return clock(t)
	case days > 0 && days < 7:
		return t.Format("Mon ") + clock(t)
	}
	return t.Format("Jan 2, ") + clock(t)
}

// calendarDays is how many dates from a's to b's, in a's zone.
func calendarDays(a, b time.Time) int {
	b = b.In(a.Location())
	da := time.Date(a.Year(), a.Month(), a.Day(), 12, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 12, 0, 0, 0, time.UTC)
	return int(db.Sub(da).Hours() / 24)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// status is how a menu stands now, for its card.
func status(m Menu, live Menu, now time.Time, loc *time.Location) (text string, ended bool) {
	spans := m.Schedule.Spans(now, now.Add(reach), loc)
	if m.Schedule.Kind == Event {
		span, ok := m.Schedule.event(loc)
		if ok && !span.End.After(now) {
			return "Ended", true
		}
	}
	switch {
	case !m.Published:
		return "Draft · won’t show", false
	case m.ID == live.ID:
		return "Live", false
	case len(spans) == 0:
		return "Not scheduled", false
	}
	next := spans[0].Start
	if m.Schedule.Kind == Event {
		switch days := calendarDays(now, next); {
		case !next.After(now):
			return "On now", false
		case days == 0:
			return "Today, " + clock(next), false
		case days == 1:
			return "Tomorrow, " + clock(next), false
		default:
			return fmt.Sprintf("In %d days", days), false
		}
	}
	if !next.After(now) && len(spans) > 1 {
		next = spans[1].Start
	}
	return "Next: " + moment(next, now), false
}

func (c Controller) Menus(w http.ResponseWriter, r *http.Request) error {
	menus, err := All(r.Context(), c.DB)
	if err != nil {
		return err
	}
	now, loc := time.Now().In(c.zone()), c.zone()
	timeline := NewTimeline(menus, now, loc)
	live, _ := timeline.At(now)
	p := menusPage{Live: live.Name, Deleted: r.URL.Query().Get("deleted") == "1"}
	if at, next, ok := timeline.Change(now); ok {
		p.Until = "until " + moment(at, now) + ", then " + next.Name
	}
	var events []Menu
	for _, m := range menus {
		if m.Schedule.Kind == Event {
			events = append(events, m)
			continue
		}
		text, _ := status(m, live, now, loc)
		p.Recurring = append(p.Recurring, menuCard{ID: m.ID, Name: m.Name, Line: m.Schedule.Summary(loc) + " · " + plural(m.Items(), "item", "items"), Status: text, Live: m.ID == live.ID && m.Published, Draft: !m.Published})
	}
	// Events in date order; ended ones, most recent first.
	slices.SortFunc(events, func(a, b Menu) int { return strings.Compare(a.Schedule.Start, b.Schedule.Start) })
	for _, m := range events {
		text, ended := status(m, live, now, loc)
		card := menuCard{ID: m.ID, Name: m.Name, Line: m.Schedule.Summary(loc) + " · " + plural(m.Items(), "item", "items"), Status: text, Live: m.ID == live.ID && m.Published && !ended, Draft: !m.Published, Ended: ended}
		if ended {
			p.Ended = append([]menuCard{card}, p.Ended...)
		} else {
			p.Events = append(p.Events, card)
		}
	}
	n := 0
	for _, list := range [][]menuCard{p.Recurring, p.Events, p.Ended} {
		for i := range list {
			n++
			list[i].Number = fmt.Sprintf("%02d", n)
		}
	}
	p.Count = plural(len(menus), "menu", "menus") + " · " + fmt.Sprint(len(p.Recurring)) + " recurring · " + plural(len(events), "one-off event", "one-off events")
	return page(w, r, http.StatusOK, menusView(p))
}

// Create starts a new menu: a draft with the live menu's sections, empty,
// for the editor to fill and schedule.
func (c Controller) Create(w http.ResponseWriter, r *http.Request) error {
	menus, err := All(r.Context(), c.DB)
	if err != nil {
		return err
	}
	m := Menu{Name: "New menu", Schedule: Schedule{Kind: Weekly, Hours: []Hours{}}}
	if live, ok := NewTimeline(menus, time.Now(), c.zone()).At(time.Now()); ok {
		m.Brand = live.Brand
		for _, s := range live.Sections {
			m.Sections = append(m.Sections, Section{Name: s.Name, Extras: []string{}, Items: []Item{}})
		}
	} else {
		if err := json.Unmarshal(Initial, &m.Content); err != nil {
			return err
		}
		for i := range m.Sections {
			m.Sections[i].Items = []Item{}
		}
	}
	m.Title = ""
	id, err := Create(r.Context(), c.DB, m)
	if err != nil {
		return err
	}
	web.Redirect(w, r, "/admin/menus/"+web.IDText(id))
	return nil
}

// Duplicate copies a menu, as a draft so it doesn't show alongside the
// original until it's rescheduled and published.
func (c Controller) Duplicate(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB, web.ID(r, "id"))
	if err != nil {
		return err
	}
	name := []rune("Copy of " + m.Name)
	if len(name) > 60 {
		name = name[:60]
	}
	m.Name, m.Published = string(name), false
	id, err := Create(r.Context(), c.DB, m)
	if err != nil {
		return err
	}
	web.Redirect(w, r, "/admin/menus/"+web.IDText(id))
	return nil
}

func (c Controller) Delete(w http.ResponseWriter, r *http.Request) error {
	if err := Delete(r.Context(), c.DB, web.ID(r, "id")); err != nil {
		if errors.Is(err, ErrLastMenu) {
			return web.Status(422, err)
		}
		return err
	}
	web.Redirect(w, r, "/admin?deleted=1")
	return nil
}

type editPage struct {
	Menu   Menu
	Status string // "Live", "Draft", …
}

func (c Controller) Editor(w http.ResponseWriter, r *http.Request) error {
	menus, err := All(r.Context(), c.DB)
	if err != nil {
		return err
	}
	id := web.ID(r, "id")
	for _, m := range menus {
		if m.ID == id {
			now := time.Now().In(c.zone())
			live, _ := NewTimeline(menus, now, c.zone()).At(now)
			text, _ := status(m, live, now, c.zone())
			return page(w, r, http.StatusOK, editor(editPage{Menu: m, Status: text}))
		}
	}
	return web.Status(404, errors.New("No such menu."))
}
func (c Controller) Read(w http.ResponseWriter, r *http.Request) error {
	m, err := Load(r.Context(), c.DB, web.ID(r, "id"))
	if err != nil {
		return err
	}
	return respond(w, m)
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return web.Status(400, errors.New("The menu could not be read."))
	}
	return nil
}

func (c Controller) Save(w http.ResponseWriter, r *http.Request) error {
	var m Menu
	if err := decode(w, r, &m); err != nil {
		return err
	}
	m.ID = web.ID(r, "id")
	if err := m.Validate(); err != nil {
		return web.Status(422, err)
	}
	if err := Save(r.Context(), c.DB, m); err != nil {
		if errors.Is(err, ErrConflict) {
			return web.Status(409, err)
		}
		return err
	}
	m.Revision++
	return respond(w, m)
}

// checkResult is a draft schedule in words, and how it sits with the
// other menus: for the editor to show as it changes.
type checkResult struct {
	Summary string       `json:"summary"`
	Error   string       `json:"error,omitempty"`
	Overlap *overlapView `json:"overlap,omitempty"`
	Day     []dayBlock   `json:"day"` // an event's first day: the other menus on it
}
type overlapView struct {
	Short string  `json:"short"` // "overlaps Regular Menu"
	Title string  `json:"title"` // "This overlaps Regular Menu from 7:00 AM to 2:00 PM."
	Note  string  `json:"note"`  // who shows during it
	Fix   *fix    `json:"fix,omitempty"`
	From  float64 `json:"from"` // on an event's first day, in hours, for its timeline
	To    float64 `json:"to"`
}
type fix struct {
	Label string    `json:"label"`
	Times [2]string `json:"times"` // the event's new start and end
}
type dayBlock struct {
	Name string  `json:"name"`
	From float64 `json:"from"` // hours after midnight
	To   float64 `json:"to"`
}

// hours is t as hours after day's midnight.
func hours(t, day time.Time) float64 { return t.Sub(day).Hours() }

// Check describes a draft of the menu's schedule: its summary, a problem
// with it, and the first time it overlaps another published menu.
func (c Controller) Check(w http.ResponseWriter, r *http.Request) error {
	var draft struct {
		Name     string   `json:"name"`
		Schedule Schedule `json:"schedule"`
	}
	if err := decode(w, r, &draft); err != nil {
		return err
	}
	menus, err := All(r.Context(), c.DB)
	if err != nil {
		return err
	}
	loc, now := c.zone(), time.Now().In(c.zone())
	m := Menu{ID: web.ID(r, "id"), Name: strings.TrimSpace(draft.Name), Schedule: draft.Schedule, Published: true}
	if m.Name == "" {
		m.Name = "This menu"
	}
	result := checkResult{Summary: m.Schedule.Summary(loc), Day: []dayBlock{}}
	if err := m.Schedule.Validate(); err != nil {
		result.Error = err.Error()
		return respond(w, result)
	}
	from, to := now, now.Add(366*24*time.Hour)
	span, isEvent := m.Schedule.event(loc)
	var day time.Time
	if isEvent {
		from, to = span.Start, span.End
		day = time.Date(span.Start.Year(), span.Start.Month(), span.Start.Day(), 0, 0, 0, 0, loc)
		for _, other := range menus {
			if other.ID == m.ID || !other.Published {
				continue
			}
			for _, s := range other.Schedule.Spans(day, day.AddDate(0, 0, 1), loc) {
				result.Day = append(result.Day, dayBlock{Name: other.Name, From: max(0, hours(s.Start, day)), To: min(24, hours(s.End, day))})
			}
		}
	}
	overlaps := Overlaps(m, menus, from, to, loc)
	if len(overlaps) == 0 {
		return respond(w, result)
	}
	o := overlaps[0]
	when := clock(o.Start) + " to " + clock(o.End)
	if !isEvent || !sameDay(span.Start, span.End) {
		when = o.Start.Format("Mon, Jan 2, ") + when
	}
	more := ""
	if len(overlaps) > 1 {
		more = " (and " + plural(len(overlaps)-1, "other time", "other times") + ")"
	}
	view := &overlapView{Short: "overlaps " + o.Other.Name, Title: "This overlaps " + o.Other.Name + " from " + when + more + "."}
	if o.Ours.Start.Before(o.Theirs.Start) {
		view.Note = "If you keep the overlap, " + o.Other.Name + " takes over at " + clock(o.Theirs.Start) + ", since it starts later."
	} else {
		view.Note = "If you keep the overlap, " + m.Name + " takes over at " + clock(o.Ours.Start) + ", since it starts later."
	}
	if isEvent {
		if o.Ours.Start.Before(o.Theirs.Start) {
			view.Fix = &fix{Label: "End at " + clock(o.Theirs.Start) + " instead", Times: [2]string{m.Schedule.Start, o.Theirs.Start.Format(eventLayout)}}
		} else {
			start := o.Theirs.End
			view.Fix = &fix{Label: "Start at " + clock(start) + " instead", Times: [2]string{start.Format(eventLayout), start.Add(span.End.Sub(span.Start)).Format(eventLayout)}}
		}
		if o.Start.Before(day.AddDate(0, 0, 1)) {
			view.From, view.To = hours(o.Start, day), min(24, hours(o.End, day))
		}
	}
	result.Overlap = view
	return respond(w, result)
}
