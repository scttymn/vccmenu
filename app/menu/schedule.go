package menu

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	// Hosts and go test have no zoneinfo file unless the binary is built
	// with -tags=timetzdata. Active still has to resolve America/Chicago.
	_ "time/tzdata"
)

type whenKind int

const (
	kindUnset whenKind = iota
	kindWeekly
	kindOneOff
)

type When struct {
	kind   whenKind
	weekly weekly
	once   oneOff
}

type weekly struct {
	days  [7]bool
	start Clock
	end   Clock
}

type oneOff struct {
	first Date
	last  Date
	from  Clock
	to    Clock
}

type Clock struct{ minute int }

type Date struct {
	y int
	m time.Month
	d int
}

type Zone struct {
	name string
	loc  *time.Location
}

func (w When) IsZero() bool { return w.kind == kindUnset }

func (z Zone) Name() string { return z.name }

func (z Zone) Location() *time.Location { return z.loc }

func ParseZone(name string) (Zone, error) {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return Zone{}, fmt.Errorf("unknown timezone %q", name)
	}
	return Zone{name: name, loc: loc}, nil
}

func ParseClock(s string) (Clock, error) {
	if s == "24:00" {
		return Clock{minute: 24 * 60}, nil
	}
	if len(s) != 5 || s[2] != ':' {
		return Clock{}, fmt.Errorf("bad clock %q", s)
	}
	parsed, err := time.Parse("15:04", s)
	if err != nil {
		return Clock{}, fmt.Errorf("bad clock %q", s)
	}
	return Clock{minute: parsed.Hour()*60 + parsed.Minute()}, nil
}

func (c Clock) String() string {
	if c.minute == 24*60 {
		return "24:00"
	}
	return fmt.Sprintf("%02d:%02d", c.minute/60, c.minute%60)
}

func ParseDate(s string) (Date, error) {
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("bad date %q", s)
	}
	return Date{y: parsed.Year(), m: parsed.Month(), d: parsed.Day()}, nil
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.y, int(d.m), d.d)
}

func (d Date) equal(o Date) bool {
	return d.y == o.y && d.m == o.m && d.d == o.d
}

func (d Date) before(o Date) bool {
	if d.y != o.y {
		return d.y < o.y
	}
	if d.m != o.m {
		return d.m < o.m
	}
	return d.d < o.d
}

func (d Date) addDays(n int) Date {
	t := time.Date(d.y, d.m, d.d, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n)
	return Date{y: t.Year(), m: t.Month(), d: t.Day()}
}

func Weekly(days []time.Weekday, start, end Clock) (When, error) {
	if start.minute < 0 || start.minute >= 24*60 {
		return When{}, newLimit("A start time has to be 23:59 or earlier.")
	}
	if end.minute < 0 || end.minute > 24*60 {
		return When{}, newLimit("An end time has to be 24:00 or earlier.")
	}
	if start.minute == end.minute {
		return When{}, newLimit("The end time has to be different from the start.")
	}
	if len(days) == 0 {
		return When{}, newLimit("Choose at least one day.")
	}
	var set [7]bool
	for _, day := range days {
		if day < time.Sunday || day > time.Saturday {
			return When{}, newLimit("Choose a real day of the week.")
		}
		set[day] = true
	}
	return When{kind: kindWeekly, weekly: weekly{days: set, start: start, end: end}}, nil
}

func OneOff(first, last Date, from, to Clock) (When, error) {
	if from.minute < 0 || from.minute >= 24*60 {
		return When{}, newLimit("A start time has to be 23:59 or earlier.")
	}
	if to.minute < 0 || to.minute > 24*60 {
		return When{}, newLimit("An end time has to be 24:00 or earlier.")
	}
	if last.before(first) {
		return When{}, newLimit("The end date has to be on or after the start date.")
	}
	end := last
	if first.equal(last) && to.minute < from.minute {
		end = last.addDays(1)
	}
	if !endsAfter(first, from.minute, end, to.minute) {
		return When{}, newLimit("The end of a one-off menu has to be after the start.")
	}
	return When{kind: kindOneOff, once: oneOff{first: first, last: end, from: from, to: to}}, nil
}

func endsAfter(start Date, startMin int, end Date, endMin int) bool {
	if start.equal(end) {
		return endMin > startMin
	}
	return start.before(end)
}

func NewMenuID() (MenuID, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return MenuID(hex.EncodeToString(buf[:])), nil
}

type Snapshot struct {
	Brand     string     `json:"brand"`
	Title     string     `json:"title"`
	ServerNow time.Time  `json:"serverNow"`
	Left      [3]Section `json:"left"`
	Timeline  []Span     `json:"timeline"`
}

type Span struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Panel *Panel    `json:"panel"`
}

type Panel struct {
	Name   string  `json:"name"`
	Combos []Combo `json:"combos"`
}

type ranked struct {
	index int
	id    MenuID
	kind  whenKind
	start time.Time
}

func (b Board) Active(instant time.Time) (MenuID, bool) {
	loc := b.Zone.Location()
	if loc == nil {
		return "", false
	}
	var best *ranked
	for i, daypart := range b.Dayparts {
		if len(daypart.Specials) == 0 {
			continue
		}
		ok, start := daypart.When.contains(instant, loc)
		if !ok {
			continue
		}
		cand := ranked{index: i, id: daypart.ID, kind: daypart.When.kind, start: start}
		if best == nil || outranks(cand, *best) {
			best = &cand
		}
	}
	if best == nil {
		return "", false
	}
	return best.id, true
}

func outranks(challenger, current ranked) bool {
	if challenger.kind != current.kind {
		return challenger.kind == kindOneOff
	}
	if challenger.start.After(current.start) {
		return true
	}
	if challenger.start.Before(current.start) {
		return false
	}
	return challenger.index < current.index
}

func (w When) contains(instant time.Time, loc *time.Location) (bool, time.Time) {
	switch w.kind {
	case kindWeekly:
		if !w.weekly.containsClock(instant, loc) {
			return false, time.Time{}
		}
		return true, w.weekly.occurrenceStart(instant, loc)
	case kindOneOff:
		if !w.once.contains(instant, loc) {
			return false, time.Time{}
		}
		return true, w.once.startInstant(loc)
	default:
		return false, time.Time{}
	}
}

func civilSeconds(t time.Time) int {
	return t.Hour()*3600 + t.Minute()*60 + t.Second()
}

func (w weekly) containsClock(instant time.Time, loc *time.Location) bool {
	local := instant.In(loc)
	sec := civilSeconds(local)
	start := w.start.minute * 60
	end := w.end.minute * 60
	today := local.Weekday()
	if end > start {
		return w.days[today] && sec >= start && sec < end
	}
	if w.days[today] && sec >= start {
		return true
	}
	yesterday := today - 1
	if today == time.Sunday {
		yesterday = time.Saturday
	}
	return w.days[yesterday] && sec < end
}

func (w weekly) occurrenceStart(instant time.Time, loc *time.Location) time.Time {
	local := instant.In(loc)
	y, m, d := local.Date()
	if w.end.minute < w.start.minute && civilSeconds(local) < w.end.minute*60 {
		y, m, d = shiftDate(y, m, d, -1)
	}
	best, ok := latestAtOrBefore(clockOnDate(loc, y, m, d, w.start.minute/60, w.start.minute%60), instant)
	if !ok {
		return instant
	}
	return best
}

func (o oneOff) contains(instant time.Time, loc *time.Location) bool {
	start := earliestCivil(loc, o.first, o.from.minute)
	end := earliestCivil(loc, o.last, o.to.minute)
	return !instant.Before(start) && instant.Before(end)
}

func (o oneOff) startInstant(loc *time.Location) time.Time {
	return earliestCivil(loc, o.first, o.from.minute)
}

func earliestCivil(loc *time.Location, date Date, minute int) time.Time {
	if minute >= 24*60 {
		midnight := time.Date(date.y, date.m, date.d, 0, 0, 0, 0, loc)
		return midnight.AddDate(0, 0, 1)
	}
	cands := clockOnDate(loc, date.y, date.m, date.d, minute/60, minute%60)
	if len(cands) == 0 {
		return time.Date(date.y, date.m, date.d, minute/60, minute%60, 0, 0, loc)
	}
	best := cands[0]
	for _, cand := range cands[1:] {
		if cand.Before(best) {
			best = cand
		}
	}
	return best
}

func clockOnDate(loc *time.Location, y int, m time.Month, d, hour, min int) []time.Time {
	base := time.Date(y, m, d, hour, min, 0, 0, loc)
	var out []time.Time
	for _, cand := range []time.Time{base.Add(-time.Hour), base, base.Add(time.Hour)} {
		local := cand.In(loc)
		if local.Year() != y || local.Month() != m || local.Day() != d || local.Hour() != hour || local.Minute() != min || local.Second() != 0 {
			continue
		}
		if !containsInstant(out, cand) {
			out = append(out, cand)
		}
	}
	return out
}

func latestAtOrBefore(cands []time.Time, instant time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	for _, cand := range cands {
		if cand.After(instant) {
			continue
		}
		if !found || cand.After(best) {
			best = cand
			found = true
		}
	}
	return best, found
}

func containsInstant(list []time.Time, instant time.Time) bool {
	for _, item := range list {
		if item.Equal(instant) {
			return true
		}
	}
	return false
}

func shiftDate(y int, m time.Month, d, days int) (int, time.Month, int) {
	t := time.Date(y, m, d, 12, 0, 0, 0, time.UTC).AddDate(0, 0, days)
	return t.Date()
}

func (b Board) Project(now time.Time) (Snapshot, error) {
	loc := b.Zone.Location()
	if loc == nil {
		return Snapshot{}, errors.New("Choose a timezone.")
	}
	local := now.In(loc)
	start := local.Truncate(time.Minute).Add(-2 * time.Minute)
	year, month, day := local.Date()
	horizon := time.Date(year, month, day, 0, 0, 0, 0, loc).AddDate(0, 0, 8)
	snap := Snapshot{
		Brand:     b.Brand,
		Title:     b.Title,
		ServerNow: now.UTC(),
		Left:      visibleSections(b.AllDay.Sections),
		Timeline:  make([]Span, 0, 8),
	}
	// Step real minutes. Jumping from one civil 1:30 to the next skips the
	// second fall-back hour, which is still 1:30 an hour later.
	var current *Span
	var currentKey string
	for at := start; at.Before(horizon); at = at.Add(time.Minute) {
		panel, key := b.rail(at)
		if current != nil && key == currentKey {
			continue
		}
		if current != nil {
			current.End = at.UTC()
			snap.Timeline = append(snap.Timeline, *current)
		}
		current = &Span{Start: at.UTC(), Panel: panel}
		currentKey = key
	}
	if current == nil {
		panel, _ := b.rail(start)
		snap.Timeline = append(snap.Timeline, Span{Start: start.UTC(), End: horizon.UTC(), Panel: panel})
		return snap, nil
	}
	current.End = horizon.UTC()
	snap.Timeline = append(snap.Timeline, *current)
	return snap, nil
}

func (b Board) rail(at time.Time) (*Panel, string) {
	if id, ok := b.Active(at); ok {
		for _, daypart := range b.Dayparts {
			if daypart.ID == id {
				return panelFrom(daypart.Name, daypart.Specials), "daypart:" + string(id)
			}
		}
	}
	if len(b.AllDay.Specials) == 0 {
		return nil, "none"
	}
	return panelFrom(b.AllDay.Name, b.AllDay.Specials), "allday"
}

func panelFrom(name string, combos []Combo) *Panel {
	copied := append([]Combo(nil), combos...)
	sort.SliceStable(copied, func(i, j int) bool { return copied[i].Number < copied[j].Number })
	return &Panel{Name: name, Combos: copied}
}

func visibleSections(in [3]Section) [3]Section {
	var out [3]Section
	for i, section := range in {
		items := make([]Item, 0, len(section.Items))
		for _, item := range section.Items {
			if item.Visible {
				items = append(items, item)
			}
		}
		extras := append([]string(nil), section.Extras...)
		if extras == nil {
			extras = []string{}
		}
		out[i] = Section{Name: section.Name, Note: section.Note, Footer: section.Footer, Extras: extras, Items: items}
	}
	return out
}

var weekdayName = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

func (w weekly) dayNames() []string {
	names := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	out := make([]string, 0, 7)
	for i, on := range w.days {
		if on {
			out = append(out, names[i])
		}
	}
	return out
}
