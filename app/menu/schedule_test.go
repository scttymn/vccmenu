package menu_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/scttymn/vccmenu/app/menu"
)

func zone(t *testing.T) menu.Zone {
	t.Helper()
	z, err := menu.ParseZone("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	return z
}

func clock(t *testing.T, s string) menu.Clock {
	t.Helper()
	c, err := menu.ParseClock(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func date(t *testing.T, s string) menu.Date {
	t.Helper()
	d, err := menu.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func weekly(t *testing.T, days []time.Weekday, start, end string) menu.When {
	t.Helper()
	w, err := menu.Weekly(days, clock(t, start), clock(t, end))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func oneOff(t *testing.T, first, last, from, to string) menu.When {
	t.Helper()
	w, err := menu.OneOff(date(t, first), date(t, last), clock(t, from), clock(t, to))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func combo(number int, name, price string) menu.Combo {
	return menu.Combo{Number: number, Name: name, Contents: name, Price: price}
}

func namedSections(name string, items ...menu.Item) [3]menu.Section {
	var sections [3]menu.Section
	sections[0] = menu.Section{Name: name, Items: items, Extras: []string{}}
	sections[1] = menu.Section{Name: "Food", Items: []menu.Item{}, Extras: []string{}}
	sections[2] = menu.Section{Name: "Ice Cream", Items: []menu.Item{}, Extras: []string{}}
	return sections
}

func emptySections() [3]menu.Section {
	var sections [3]menu.Section
	for i := range sections {
		sections[i] = menu.Section{Items: []menu.Item{}, Extras: []string{}}
	}
	return sections
}

func daypart(id, name, price string, when menu.When) menu.Daypart {
	return menu.Daypart{
		MenuBody: menu.MenuBody{
			ID:       menu.MenuID(id),
			Name:     name,
			Sections: emptySections(),
			Specials: []menu.Combo{combo(1, name, price)},
		},
		When: when,
	}
}

func chicagoAt(z menu.Zone, y int, m time.Month, d, hour, min, sec int) time.Time {
	return time.Date(y, m, d, hour, min, sec, 0, z.Location())
}

func priceOf(board menu.Board, id menu.MenuID) string {
	body, ok := findBody(board, id)
	if !ok || len(body.Specials) == 0 {
		return ""
	}
	return body.Specials[0].Price
}

func findBody(board menu.Board, id menu.MenuID) (menu.MenuBody, bool) {
	if board.AllDay.ID == id {
		return board.AllDay, true
	}
	for _, part := range board.Dayparts {
		if part.ID == id {
			return part.MenuBody, true
		}
	}
	return menu.MenuBody{}, false
}

func mondayBoard(t *testing.T) menu.Board {
	t.Helper()
	z := zone(t)
	return menu.Board{
		Brand: "Valley Coffee Co",
		Title: "Menu",
		Zone:  z,
		AllDay: menu.MenuBody{
			ID:       "all-day",
			Name:     "All day",
			Sections: namedSections("Drinks", menu.Item{Name: "Drip", Price: "$3", Visible: true}),
			Specials: []menu.Combo{combo(1, "House", "$4")},
		},
		Dayparts: []menu.Daypart{
			daypart("breakfast", "Breakfast", "$6", weekly(t, []time.Weekday{time.Monday}, "06:00", "10:30")),
			daypart("lunch", "Lunch", "$8", weekly(t, []time.Weekday{time.Monday}, "10:30", "14:00")),
		},
	}
}

func winner(t *testing.T, board menu.Board, at time.Time, id, price string) {
	t.Helper()
	got, ok := board.Active(at)
	if !ok || got != menu.MenuID(id) || priceOf(board, got) != price {
		t.Fatalf("at %s winner %q ok=%v price %q, want %s %s", at.Format(time.RFC3339), got, ok, priceOf(board, got), id, price)
	}
}

func TestActiveBoundaryBreakfastThenLunch(t *testing.T) {
	board := mondayBoard(t)
	z := board.Zone
	monday := chicagoAt(z, 2026, 10, 5, 10, 29, 59)
	if monday.Weekday() != time.Monday {
		t.Fatalf("fixture weekday %s", monday.Weekday())
	}
	winner(t, board, monday, "breakfast", "$6")
	winner(t, board, chicagoAt(z, 2026, 10, 5, 10, 30, 0), "lunch", "$8")
	sunday := chicagoAt(z, 2026, 10, 4, 7, 0, 0)
	if got, ok := board.Active(sunday); ok {
		t.Fatalf("sunday selected %s at %s", got, priceOf(board, got))
	}
	utc := time.Date(2026, 10, 5, 15, 30, 0, 0, time.UTC)
	local := chicagoAt(z, 2026, 10, 5, 10, 30, 0)
	if !utc.Equal(local) {
		t.Fatalf("10:30 Chicago is %s", local.UTC().Format(time.RFC3339))
	}
	winner(t, board, utc, "lunch", "$8")
}

func TestActiveOneOffBeatsWeekly(t *testing.T) {
	board := mondayBoard(t)
	board.Dayparts = append(board.Dayparts, daypart("market", "Market", "$5", oneOff(t, "2026-10-05", "2026-10-05", "00:00", "24:00")))
	winner(t, board, chicagoAt(board.Zone, 2026, 10, 5, 10, 30, 0), "market", "$5")
}

func TestActiveLaterStartAndEarlierIndex(t *testing.T) {
	z := zone(t)
	at := chicagoAt(z, 2026, 10, 5, 9, 30, 0)
	laterFirst := menu.Board{Zone: z, Dayparts: []menu.Daypart{
		daypart("late", "Late", "$9", weekly(t, []time.Weekday{time.Monday}, "09:00", "10:30")),
		daypart("early", "Early", "$6", weekly(t, []time.Weekday{time.Monday}, "06:00", "11:00")),
	}}
	winner(t, laterFirst, at, "late", "$9")
	laterSecond := menu.Board{Zone: z, Dayparts: []menu.Daypart{
		daypart("early", "Early", "$6", weekly(t, []time.Weekday{time.Monday}, "06:00", "11:00")),
		daypart("late", "Late", "$9", weekly(t, []time.Weekday{time.Monday}, "09:00", "10:30")),
	}}
	winner(t, laterSecond, at, "late", "$9")
	sameStart := menu.Board{Zone: z, Dayparts: []menu.Daypart{
		daypart("first", "First", "$6", weekly(t, []time.Weekday{time.Monday}, "09:00", "11:00")),
		daypart("second", "Second", "$7", weekly(t, []time.Weekday{time.Monday}, "09:00", "11:00")),
	}}
	winner(t, sameStart, at, "first", "$6")
}

func TestActiveSkipsDaypartWithNoCombos(t *testing.T) {
	board := mondayBoard(t)
	board.Dayparts[1].Specials = nil
	at := chicagoAt(board.Zone, 2026, 10, 5, 12, 0, 0)
	if got, ok := board.Active(at); ok {
		t.Fatalf("empty lunch selected %s at %s", got, priceOf(board, got))
	}
	board.Dayparts = append(board.Dayparts, menu.Daypart{
		MenuBody: menu.MenuBody{ID: "copy", Name: "Copy", Sections: emptySections()},
		When:     board.Dayparts[0].When,
	})
	winner(t, board, chicagoAt(board.Zone, 2026, 10, 5, 9, 0, 0), "breakfast", "$6")
}

func TestActiveOvernightWeekly(t *testing.T) {
	z := zone(t)
	board := menu.Board{Zone: z, Dayparts: []menu.Daypart{
		daypart("late", "Late", "$4", weekly(t, []time.Weekday{time.Monday}, "21:00", "01:00")),
	}}
	winner(t, board, chicagoAt(z, 2026, 10, 5, 21, 30, 0), "late", "$4")
	winner(t, board, chicagoAt(z, 2026, 10, 6, 0, 30, 0), "late", "$4")
	if got, ok := board.Active(chicagoAt(z, 2026, 10, 6, 1, 0, 0)); ok {
		t.Fatalf("01:00 selected %s", got)
	}
}

func TestActiveRepeatedHour(t *testing.T) {
	z := zone(t)
	board := menu.Board{Zone: z, Dayparts: []menu.Daypart{
		daypart("early", "Early", "$3", weekly(t, []time.Weekday{time.Sunday}, "01:30", "01:45")),
	}}
	first, second := bothOneThirty(t, z.Location())
	winner(t, board, first, "early", "$3")
	winner(t, board, second, "early", "$3")
	if got, ok := board.Active(first.Add(20 * time.Minute)); ok {
		t.Fatalf("01:50 selected %s at %s", got, priceOf(board, got))
	}
}

func bothOneThirty(t *testing.T, loc *time.Location) (time.Time, time.Time) {
	t.Helper()
	start := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	var hits []time.Time
	for at := start; at.Before(start.Add(4 * time.Hour)); at = at.Add(time.Minute) {
		local := at.In(loc)
		if local.Hour() == 1 && local.Minute() == 30 && local.Second() == 0 {
			hits = append(hits, at)
		}
	}
	if len(hits) != 2 {
		t.Fatalf("expected two 1:30 instants, got %d", len(hits))
	}
	if hits[0].Equal(hits[1]) || hits[0].In(loc).Format("-07:00") == hits[1].In(loc).Format("-07:00") {
		t.Fatalf("1:30 instants are not the two fall-back clocks: %s %s", hits[0], hits[1])
	}
	return hits[0], hits[1]
}

func TestParseStoredLegacyAndParseEditorRejectsIt(t *testing.T) {
	board, err := menu.ParseStored(menu.Initial)
	if err != nil {
		t.Fatal(err)
	}
	if board.Zone.Name() != "America/Chicago" || board.AllDay.ID != "all-day" || board.AllDay.Name != "All day" || len(board.Dayparts) != 0 || len(board.AllDay.Specials) != 0 {
		t.Fatalf("legacy board: zone %s id %s name %s dayparts %d specials %d", board.Zone.Name(), board.AllDay.ID, board.AllDay.Name, len(board.Dayparts), len(board.AllDay.Specials))
	}
	drinks := board.AllDay.Sections[0]
	if drinks.Name != "Drinks" || drinks.Items[0].Name != "Latte" || drinks.Items[0].Price != "$5.5 / $6 / $6.5" || drinks.Items[1].Name != "Americano" || drinks.Items[1].Price != "$3 / $3.5 / $4" {
		t.Fatalf("drinks: %+v", drinks.Items[:2])
	}
	food := board.AllDay.Sections[1]
	if food.Name != "Food" || food.Items[0].Name != "Chocolate Chip Cookie" || food.Items[0].Price != "$3.99" {
		t.Fatalf("food: %+v", food.Items[0])
	}
	ice := board.AllDay.Sections[2]
	if ice.Name != "Ice Cream" || ice.Items[0].Name != "Single Scoop" || ice.Items[0].Price != "$2" {
		t.Fatalf("ice cream: %+v", ice.Items[0])
	}
	if _, err := menu.ParseEditor(menu.Initial); err == nil {
		t.Fatal("editor accepted a legacy menu")
	}
	both := []byte(`{"brand":"Valley Coffee Co","title":"Menu","sections":[],"allDay":{"id":"all-day","name":"All day","sections":[]}}`)
	if _, err := menu.ParseStored(both); err == nil {
		t.Fatal("stored document accepted both shapes")
	}
}

func TestOneOffRangeAndOmittedTimes(t *testing.T) {
	raw := []byte(`{
		"brand":"Valley Coffee Co","title":"Menu","timezone":"America/Chicago",
		"allDay":{"id":"all-day","name":"All day","sections":[
			{"name":"Drinks","note":"","footer":"","extras":[],"items":[]},
			{"name":"Food","note":"","footer":"","extras":[],"items":[]},
			{"name":"Ice Cream","note":"","footer":"","extras":[],"items":[]}
		],"specials":[]},
		"dayparts":[{
			"id":"market","name":"Market","sections":[
				{"name":"","note":"","footer":"","extras":[],"items":[]},
				{"name":"","note":"","footer":"","extras":[],"items":[]},
				{"name":"","note":"","footer":"","extras":[],"items":[]}
			],
			"specials":[{"number":1,"name":"Stall","contents":"Coffee and a cookie","price":"$5"}],
			"when":{"kind":"oneOff","first":"2026-10-05","last":"2026-10-05","from":"21:00","to":"01:00"}
		}]
	}`)
	board, err := menu.ParseEditor(raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(board.Dayparts[0].When)
	if err != nil {
		t.Fatal(err)
	}
	var when map[string]string
	if err := json.Unmarshal(encoded, &when); err != nil {
		t.Fatal(err)
	}
	if when["last"] != "2026-10-06" || when["to"] != "01:00" || when["from"] != "21:00" {
		t.Fatalf("overnight window: %s", encoded)
	}
	winner(t, board, chicagoAt(board.Zone, 2026, 10, 5, 22, 0, 0), "market", "$5")
	winner(t, board, chicagoAt(board.Zone, 2026, 10, 6, 0, 30, 0), "market", "$5")
	if got, ok := board.Active(chicagoAt(board.Zone, 2026, 10, 6, 1, 0, 0)); ok {
		t.Fatalf("one-off end selected %s", got)
	}

	day := []byte(`{
		"brand":"Valley Coffee Co","title":"Menu","timezone":"America/Chicago",
		"allDay":{"id":"all-day","name":"All day","sections":[
			{"name":"Drinks","note":"","footer":"","extras":[],"items":[]},
			{"name":"Food","note":"","footer":"","extras":[],"items":[]},
			{"name":"Ice Cream","note":"","footer":"","extras":[],"items":[]}
		],"specials":[{"number":1,"name":"House","contents":"Drip","price":"$4"}]},
		"dayparts":[{
			"id":"fair","name":"Fair","sections":[
				{"name":"","note":"","footer":"","extras":[],"items":[]},
				{"name":"","note":"","footer":"","extras":[],"items":[]},
				{"name":"","note":"","footer":"","extras":[],"items":[]}
			],
			"specials":[{"number":2,"name":"Booth","contents":"Cocoa","price":"$3"}],
			"when":{"kind":"oneOff","first":"2026-10-05","last":"2026-10-05"}
		}]
	}`)
	full, err := menu.ParseEditor(day)
	if err != nil {
		t.Fatal(err)
	}
	winner(t, full, chicagoAt(full.Zone, 2026, 10, 5, 0, 0, 0), "fair", "$3")
	winner(t, full, chicagoAt(full.Zone, 2026, 10, 5, 23, 59, 0), "fair", "$3")
	if got, ok := full.Active(chicagoAt(full.Zone, 2026, 10, 6, 0, 0, 0)); ok {
		t.Fatalf("next midnight selected %s at %s", got, priceOf(full, got))
	}
}

func TestWindowConstructorsRejectEmptyRanges(t *testing.T) {
	if _, err := menu.Weekly(nil, clock(t, "06:00"), clock(t, "10:00")); err == nil {
		t.Fatal("empty days")
	}
	if _, err := menu.Weekly([]time.Weekday{time.Monday}, clock(t, "10:00"), clock(t, "10:00")); err == nil {
		t.Fatal("equal clocks")
	}
	if _, err := menu.Weekly([]time.Weekday{time.Monday}, clock(t, "24:00"), clock(t, "10:00")); err == nil {
		t.Fatal("start at 24:00")
	}
	if _, err := menu.OneOff(date(t, "2026-10-06"), date(t, "2026-10-05"), clock(t, "00:00"), clock(t, "24:00")); err == nil {
		t.Fatal("backwards dates")
	}
}

func spanAt(t *testing.T, snap menu.Snapshot, at time.Time) (menu.Span, menu.Span) {
	t.Helper()
	for i, span := range snap.Timeline {
		if !at.Before(span.Start) && at.Before(span.End) {
			var next menu.Span
			if i+1 < len(snap.Timeline) {
				next = snap.Timeline[i+1]
			}
			return span, next
		}
	}
	t.Fatalf("no span contains %s", at.Format(time.RFC3339))
	return menu.Span{}, menu.Span{}
}

func assertContinuous(t *testing.T, snap menu.Snapshot, loc *time.Location, now time.Time) {
	t.Helper()
	if len(snap.Timeline) == 0 {
		t.Fatal("empty timeline")
	}
	year, month, day := now.In(loc).Date()
	horizon := time.Date(year, month, day, 0, 0, 0, 0, loc).AddDate(0, 0, 8)
	last := snap.Timeline[len(snap.Timeline)-1]
	if !last.End.Equal(horizon) {
		t.Fatalf("last span ends %s, horizon %s", last.End.Format(time.RFC3339), horizon.Format(time.RFC3339))
	}
	for i := 1; i < len(snap.Timeline); i++ {
		if !snap.Timeline[i].Start.Equal(snap.Timeline[i-1].End) {
			t.Fatalf("timeline gap between %s and %s", snap.Timeline[i-1].End.Format(time.RFC3339), snap.Timeline[i].Start.Format(time.RFC3339))
		}
	}
}

func TestProjectBoundaryAndIdleRail(t *testing.T) {
	board := mondayBoard(t)
	board.AllDay.Sections[0].Items = append(board.AllDay.Sections[0].Items, menu.Item{Name: "Americano", Price: "$3", Visible: false})
	board.Dayparts[0].Sections[0].Items = []menu.Item{{Name: "Secret Scone", Price: "$1", Visible: true}}
	board.Dayparts[0].Specials = []menu.Combo{
		{Number: 2, Name: "Second", Contents: "Muffin", Price: "$2"},
		{Number: 1, Name: "Sunrise", Contents: "Drip coffee and a muffin", Price: "$6"},
	}
	at := chicagoAt(board.Zone, 2026, 10, 5, 10, 29, 0)
	snap, err := board.Project(at)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.ServerNow.Equal(at.UTC()) {
		t.Fatalf("serverNow %s", snap.ServerNow.Format(time.RFC3339))
	}
	assertContinuous(t, snap, board.Zone.Location(), at)
	span, next := spanAt(t, snap, chicagoAt(board.Zone, 2026, 10, 5, 10, 29, 59))
	boundary := chicagoAt(board.Zone, 2026, 10, 5, 10, 30, 0)
	if !span.End.Equal(boundary) || span.Panel == nil || span.Panel.Name != "Breakfast" || len(span.Panel.Combos) != 2 || span.Panel.Combos[0].Number != 1 || span.Panel.Combos[0].Price != "$6" || span.Panel.Combos[1].Price != "$2" {
		t.Fatalf("breakfast span ends %s panel %+v", span.End.Format(time.RFC3339), span.Panel)
	}
	if !next.Start.Equal(boundary) || next.Panel == nil || next.Panel.Name != "Lunch" || next.Panel.Combos[0].Price != "$8" {
		t.Fatalf("lunch span starts %s panel %+v", next.Start.Format(time.RFC3339), next.Panel)
	}
	if len(snap.Left[0].Items) != 1 || snap.Left[0].Items[0].Name != "Drip" || snap.Left[0].Items[0].Price != "$3" {
		t.Fatalf("left items %+v", snap.Left[0].Items)
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Secret Scone")) || bytes.Contains(raw, []byte("Americano")) {
		t.Fatal("hidden item or daypart section leaked onto the snapshot")
	}

	board.Dayparts[1].Specials = nil
	noon := chicagoAt(board.Zone, 2026, 10, 5, 12, 0, 0)
	idle, err := board.Project(noon)
	if err != nil {
		t.Fatal(err)
	}
	gap, _ := spanAt(t, idle, noon)
	if gap.Panel == nil || gap.Panel.Name != "All day" || gap.Panel.Combos[0].Name != "House" || gap.Panel.Combos[0].Price != "$4" {
		t.Fatalf("empty lunch cleared the rail: %+v", gap.Panel)
	}

	sunday := chicagoAt(board.Zone, 2026, 10, 4, 7, 0, 0)
	off, err := board.Project(sunday)
	if err != nil {
		t.Fatal(err)
	}
	offSpan, _ := spanAt(t, off, sunday)
	if offSpan.Panel == nil || offSpan.Panel.Name != "All day" || offSpan.Panel.Combos[0].Price != "$4" {
		t.Fatalf("sunday rail %+v", offSpan.Panel)
	}
}

func TestProjectRepeatedHour(t *testing.T) {
	z := zone(t)
	board := menu.Board{
		Zone: z,
		AllDay: menu.MenuBody{
			ID: "all-day", Name: "All day", Sections: namedSections("Drinks"),
			Specials: []menu.Combo{combo(1, "House", "$4")},
		},
		Dayparts: []menu.Daypart{
			daypart("early", "Early", "$3", weekly(t, []time.Weekday{time.Sunday}, "01:30", "01:45")),
		},
	}
	now := chicagoAt(z, 2026, 11, 1, 0, 30, 0)
	snap, err := board.Project(now)
	if err != nil {
		t.Fatal(err)
	}
	assertContinuous(t, snap, z.Location(), now)
	cutoff := chicagoAt(z, 2026, 11, 1, 3, 0, 0)
	var morning []menu.Span
	for _, span := range snap.Timeline {
		if span.Panel != nil && span.Panel.Name == "Early" && span.Start.Before(cutoff) {
			morning = append(morning, span)
		}
	}
	if len(morning) != 2 || !morning[0].End.Before(morning[1].Start) || morning[0].Panel.Combos[0].Price != "$3" {
		t.Fatalf("morning spans %d", len(morning))
	}
	between, _ := spanAt(t, snap, morning[0].End)
	if between.Panel == nil || between.Panel.Name != "All day" || between.Panel.Combos[0].Price != "$4" {
		t.Fatalf("gap between the repeated hour %+v", between.Panel)
	}
}

func TestCopyIsIndependent(t *testing.T) {
	board := mondayBoard(t)
	board.AllDay.Sections[0].Items[0].Price = "$5"
	board.AllDay.Sections[2].Extras = []string{"Sprinkles"}
	board.AllDay.Specials[0].Price = "$4"
	when := oneOff(t, "2026-10-09", "2026-10-09", "16:00", "20:00")
	next, id, err := board.Copy(board.AllDay.ID, "Friday market", when)
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != board.Revision || next.AllDay.ID != board.AllDay.ID || len(next.Dayparts) != len(board.Dayparts)+1 {
		t.Fatalf("copy replaced the board: revision %d all-day %s dayparts %d", next.Revision, next.AllDay.ID, len(next.Dayparts))
	}
	copied := &next.Dayparts[len(next.Dayparts)-1]
	if copied.ID != id || copied.Name != "Friday market" || copied.ID == board.AllDay.ID {
		t.Fatalf("copy id %s name %s", copied.ID, copied.Name)
	}
	gotWhen, err := json.Marshal(copied.When)
	if err != nil {
		t.Fatal(err)
	}
	wantWhen, err := json.Marshal(when)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotWhen) != string(wantWhen) {
		t.Fatalf("when %s want %s", gotWhen, wantWhen)
	}
	if copied.Sections[0].Items[0].Price != "$5" || copied.Specials[0].Price != "$4" || copied.Sections[2].Extras[0] != "Sprinkles" {
		t.Fatalf("copied item %s special %s extra %v", copied.Sections[0].Items[0].Price, copied.Specials[0].Price, copied.Sections[2].Extras)
	}
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	winner(t, next, chicagoAt(next.Zone, 2026, 10, 9, 17, 0, 0), string(id), "$4")

	next.AllDay.Sections[0].Items[0].Price = "$1"
	next.AllDay.Specials[0].Price = "$9"
	next.AllDay.Sections[2].Extras[0] = "Oreos"
	if copied.Sections[0].Items[0].Price != "$5" || copied.Specials[0].Price != "$4" || copied.Sections[2].Extras[0] != "Sprinkles" {
		t.Fatal("editing the all-day menu changed the copy")
	}
	copied.Sections[0].Items[0].Price = "$2"
	copied.Specials[0].Price = "$7"
	copied.Sections[2].Extras[0] = "Caramel"
	if next.AllDay.Sections[0].Items[0].Price != "$1" || next.AllDay.Specials[0].Price != "$9" || next.AllDay.Sections[2].Extras[0] != "Oreos" {
		t.Fatal("editing the copy changed the all-day menu")
	}
	next.Dayparts[0].Specials[0].Price = "$11"
	if board.AllDay.Sections[0].Items[0].Price != "$5" || board.AllDay.Specials[0].Price != "$4" || board.AllDay.Sections[2].Extras[0] != "Sprinkles" || board.Dayparts[0].Specials[0].Price != "$6" {
		t.Fatal("editing the result changed the input board")
	}
	if len(board.Dayparts) != 2 {
		t.Fatal("copy appended to the input board")
	}

	fromBreakfast := board.Dayparts[0].When
	friday := oneOff(t, "2026-10-09", "2026-10-09", "16:00", "18:00")
	branched, branchedID, err := board.Copy(board.Dayparts[0].ID, "Friday breakfast", friday)
	if err != nil {
		t.Fatal(err)
	}
	branch := branched.Dayparts[len(branched.Dayparts)-1]
	if branch.ID != branchedID || branch.Specials[0].Price != "$6" {
		t.Fatalf("breakfast copy %s %s", branch.ID, branch.Specials[0].Price)
	}
	branchWhen, _ := json.Marshal(branch.When)
	fridayWhen, _ := json.Marshal(friday)
	keptWhen, _ := json.Marshal(fromBreakfast)
	if string(branchWhen) != string(fridayWhen) || string(branchWhen) == string(keptWhen) {
		t.Fatalf("branch when %s", branchWhen)
	}
	if _, _, err := board.Copy(board.AllDay.ID, "Nope", menu.When{}); err == nil {
		t.Fatal("copy accepted an empty window")
	}
	if _, _, err := board.Copy("missing", "Nope", when); err == nil {
		t.Fatal("copy accepted a missing menu")
	}
}

func TestProjectRequiresZone(t *testing.T) {
	if _, err := (menu.Board{}).Project(time.Now()); err == nil {
		t.Fatal("missing zone projected a timeline")
	}
}

func TestValidateComboLimit(t *testing.T) {
	board := mondayBoard(t)
	specials := make([]menu.Combo, 0, 21)
	for n := 1; n <= 20; n++ {
		specials = append(specials, combo(n, "Combo", "$1"))
	}
	board.AllDay.Specials = specials
	if err := board.Validate(); err != nil {
		t.Fatal(err)
	}
	board.AllDay.Specials = append(board.AllDay.Specials, combo(21, "Extra", "$2"))
	if err := board.Validate(); err == nil {
		t.Fatal("21 specials")
	}
}
