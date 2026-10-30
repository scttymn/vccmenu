package menu

import (
	"slices"
	"time"
)

// reach is how far a Timeline looks either side of its moment: far enough
// back to find the last menu shown after a long season ends, and ahead to
// a menu's next showing.
const reach = 400 * 24 * time.Hour

// Timeline is when each published menu shows, around a moment, and so
// which is live. Of the menus scheduled at a time, the one that started
// last shows; between menus, the one that ended last stays up; before any
// has run, the oldest shows.
type Timeline struct {
	menus []Menu
	spans [][]Span
}

func NewTimeline(menus []Menu, around time.Time, loc *time.Location) Timeline {
	var t Timeline
	for _, m := range menus {
		if m.Published {
			t.menus = append(t.menus, m)
			t.spans = append(t.spans, m.Schedule.Spans(around.Add(-reach), around.Add(reach), loc))
		}
	}
	return t
}

// wins reports whether a showing at at, of menu i, beats one at bestAt of
// menu best: the later wins, then a one-off event, then the newer menu.
func (t Timeline) wins(i int, at time.Time, best int, bestAt time.Time) bool {
	if best < 0 || !at.Equal(bestAt) {
		return best < 0 || at.After(bestAt)
	}
	a, b := t.menus[i].Schedule.Kind == Event, t.menus[best].Schedule.Kind == Event
	if a != b {
		return a
	}
	return t.menus[i].ID > t.menus[best].ID
}

// At is the menu live at when; false when nothing is published.
func (t Timeline) At(when time.Time) (Menu, bool) {
	best, bestAt := -1, time.Time{}
	for i, spans := range t.spans {
		for _, s := range spans {
			if !s.Start.After(when) && s.End.After(when) && t.wins(i, s.Start, best, bestAt) {
				best, bestAt = i, s.Start
			}
		}
	}
	if best < 0 {
		for i, spans := range t.spans {
			for _, s := range spans {
				if !s.End.After(when) && t.wins(i, s.End, best, bestAt) {
					best, bestAt = i, s.End
				}
			}
		}
	}
	if best < 0 {
		if len(t.menus) == 0 {
			return Menu{}, false
		}
		best = 0
	}
	return t.menus[best], true
}

// Change is when the live menu next changes after when, within two weeks,
// and to which menu.
func (t Timeline) Change(when time.Time) (time.Time, Menu, bool) {
	current, ok := t.At(when)
	if !ok {
		return time.Time{}, Menu{}, false
	}
	limit := when.Add(14 * 24 * time.Hour)
	var moments []time.Time
	for _, spans := range t.spans {
		for _, s := range spans {
			for _, m := range []time.Time{s.Start, s.End} {
				if m.After(when) && !m.After(limit) {
					moments = append(moments, m)
				}
			}
		}
	}
	slices.SortFunc(moments, time.Time.Compare)
	for _, m := range moments {
		if next, _ := t.At(m); next.ID != current.ID {
			return m, next, true
		}
	}
	return time.Time{}, Menu{}, false
}

// Overlap is a time a menu and another are both scheduled.
type Overlap struct {
	Other        Menu
	Ours, Theirs Span // the two showings
	Start, End   time.Time
}

// Overlaps is every time m is scheduled alongside another published menu,
// from from to to, earliest first.
func Overlaps(m Menu, others []Menu, from, to time.Time, loc *time.Location) []Overlap {
	var out []Overlap
	ours := m.Schedule.Spans(from, to, loc)
	for _, other := range others {
		if other.ID == m.ID || !other.Published {
			continue
		}
		for _, theirs := range other.Schedule.Spans(from, to, loc) {
			for _, s := range ours {
				start, end := later(s.Start, theirs.Start), earlier(s.End, theirs.End)
				if end.After(start) {
					out = append(out, Overlap{Other: other, Ours: s, Theirs: theirs, Start: start, End: end})
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b Overlap) int { return a.Start.Compare(b.Start) })
	return out
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
