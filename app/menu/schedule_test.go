package menu

import (
	"testing"
	"time"
)

var zone = time.FixedZone("shop", -6*3600)

// at is a time in the shop's zone: Thu, Oct 29 2026 is day 29.
func at(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, zone) }

func weekly(id int64, name string, days []string, start, end string) Menu {
	return Menu{ID: id, Name: name, Published: true, Schedule: Schedule{Kind: Weekly, Hours: []Hours{{Days: days, Start: start, End: end}}}}
}
func event(id int64, name, start, end string) Menu {
	return Menu{ID: id, Name: name, Published: true, Schedule: Schedule{Kind: Event, Start: start, End: end}}
}

func TestLiveMenu(t *testing.T) {
	regular := weekly(1, "Regular", []string{"tue", "wed", "thu", "fri", "sat"}, "07:00", "14:00")
	wednesday := weekly(2, "Wednesday Night", []string{"wed"}, "17:00", "20:00")
	festival := event(3, "Fall Festival", "2026-10-31T11:00", "2026-10-31T16:00")
	draft := event(4, "Draft", "2026-10-29T08:00", "2026-10-29T09:00")
	draft.Published = false
	menus := []Menu{regular, wednesday, festival, draft}
	for _, tc := range []struct {
		when time.Time
		want string
	}{
		{at(29, 8, 30), "Regular"},                      // scheduled; the draft never shows
		{at(29, 15, 0), "Regular"},                      // between menus, the last stays up
		{at(28, 18, 0), "Wednesday Night"},              // scheduled
		{at(29, 6, 0), "Wednesday Night"},               // it ended last, the night before
		{at(31, 10, 0), "Regular"},                      // Saturday before the festival
		{at(31, 12, 0), "Fall Festival"},                // it started later, so it takes over
		{at(31, 17, 0), "Fall Festival"},                // it ended last
		{at(1, 9, 0).AddDate(0, 1, 0), "Fall Festival"}, // Sunday: nothing runs
	} {
		got, ok := NewTimeline(menus, tc.when, zone).At(tc.when)
		if !ok || got.Name != tc.want {
			t.Errorf("at %s: %q, want %q", tc.when.Format("Mon 15:04"), got.Name, tc.want)
		}
	}
	// Before anything has run, the oldest published menu shows.
	unscheduled := Menu{ID: 1, Name: "Unscheduled", Published: true, Schedule: Schedule{Kind: Weekly}}
	if got, _ := NewTimeline([]Menu{unscheduled, festival}, at(26, 9, 0), zone).At(at(26, 9, 0)); got.Name != "Unscheduled" {
		t.Errorf("oldest: %q", got.Name)
	}
	if _, ok := NewTimeline([]Menu{draft}, at(29, 8, 0), zone).At(at(29, 8, 0)); ok {
		t.Error("a draft shows when nothing is published")
	}
	when, next, ok := NewTimeline(menus, at(29, 9, 0), zone).Change(at(29, 9, 0))
	if !ok || next.Name != "Fall Festival" || !when.Equal(at(31, 11, 0)) {
		t.Errorf("change: %s %q %v", when, next.Name, ok)
	}
}

func TestOnlyMenuWithNoHoursStaysUp(t *testing.T) {
	only := Menu{ID: 1, Name: "Regular", Published: true, Schedule: Schedule{Kind: Weekly, Hours: []Hours{}}}
	if got, ok := NewTimeline([]Menu{only}, at(29, 9, 0), zone).At(at(29, 9, 0)); !ok || got.ID != 1 {
		t.Fatal("an unscheduled only menu must keep showing")
	}
}

func TestSeasonAndSummary(t *testing.T) {
	s := Schedule{Kind: Weekly, Hours: []Hours{{Days: []string{"tue", "wed", "thu", "fri", "sat"}, Start: "07:00", End: "14:00"}, {Days: []string{"sun"}, Start: "08:00", End: "12:30"}}, From: "2026-11-01", Until: "2026-11-30"}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := s.Summary(zone); got != "Tue – Sat · 7:00 AM – 2:00 PM; Sun · 8:00 AM – 12:30 PM · Nov 1 – Nov 30" {
		t.Errorf("summary: %q", got)
	}
	spans := s.Spans(at(29, 0, 0), at(29, 0, 0).AddDate(0, 0, 7), zone)
	if len(spans) == 0 || !spans[0].Start.Equal(time.Date(2026, 11, 1, 8, 0, 0, 0, zone)) {
		t.Errorf("season: %v", spans)
	}
	if got := days([]string{"mon", "wed", "thu"}); got != "Mon, Wed, Thu" {
		t.Errorf("days: %q", got)
	}
	e := Schedule{Kind: Event, Start: "2026-11-14T14:00", End: "2026-11-14T18:00"}
	if got := e.Summary(zone); got != "Sat, Nov 14 · 2:00 PM – 6:00 PM" {
		t.Errorf("event summary: %q", got)
	}
}

func TestScheduleValidation(t *testing.T) {
	for _, s := range []Schedule{
		{Kind: "monthly"},
		{Kind: Weekly, Hours: []Hours{{Days: []string{}, Start: "07:00", End: "14:00"}}},
		{Kind: Weekly, Hours: []Hours{{Days: []string{"mon"}, Start: "14:00", End: "07:00"}}},
		{Kind: Weekly, Hours: []Hours{{Days: []string{"mon"}, Start: "07:00", End: "14:00"}, {Days: []string{"mon"}, Start: "17:00", End: "20:00"}}},
		{Kind: Weekly, From: "2026-12-01", Until: "2026-11-01"},
		{Kind: Event, Start: "2026-11-14T18:00", End: "2026-11-14T14:00"},
		{Kind: Event, Start: "2026-11-14", End: "2026-11-15"},
	} {
		if s.Validate() == nil {
			t.Errorf("accepted %+v", s)
		}
	}
}

func TestOverlaps(t *testing.T) {
	regular := weekly(1, "Regular", []string{"sat"}, "07:00", "14:00")
	festival := event(3, "Fall Festival", "2026-10-31T11:00", "2026-10-31T16:00")
	got := Overlaps(festival, []Menu{regular, festival}, at(31, 0, 0), at(31, 23, 0), zone)
	if len(got) != 1 || got[0].Other.ID != 1 || !got[0].Start.Equal(at(31, 11, 0)) || !got[0].End.Equal(at(31, 14, 0)) {
		t.Fatalf("overlaps: %+v", got)
	}
	regular.Published = false
	if got := Overlaps(festival, []Menu{regular}, at(31, 0, 0), at(31, 23, 0), zone); len(got) != 0 {
		t.Fatal("a draft counts as an overlap")
	}
}
