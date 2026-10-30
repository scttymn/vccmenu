package menu

import (
	"errors"
	"slices"
	"strings"
	"time"
)

const (
	Weekly = "weekly" // the same days and hours every week, optionally only during a season
	Event  = "event"  // once, from one moment to another

	dateLayout  = "2006-01-02"
	clockLayout = "15:04"
	eventLayout = "2006-01-02T15:04"
)

// dayNames are the schedule's day names, by time.Weekday.
var dayNames = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
var dayLabels = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// Week is the days in the order editors see them.
var Week = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}

// Schedule is when a menu shows, in the shop's time zone.
type Schedule struct {
	Kind  string  `json:"kind"`
	Hours []Hours `json:"hours"` // weekly: each row's days share its hours
	From  string  `json:"from"`  // weekly: the season's first day, 2006-01-02; blank, no start
	Until string  `json:"until"` // weekly: the season's last day
	Start string  `json:"start"` // event: 2006-01-02T15:04
	End   string  `json:"end"`
}

// Hours are a weekly menu's hours on some days.
type Hours struct {
	Days  []string `json:"days"` // mon … sun
	Start string   `json:"start"`
	End   string   `json:"end"`
}

// Span is one showing, from Start until End.
type Span struct{ Start, End time.Time }

// minutes is a clock time, 15:04, as minutes after midnight.
func minutes(v string) (int, bool) {
	t, err := time.Parse(clockLayout, v)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

func (s Schedule) event(loc *time.Location) (Span, bool) {
	start, err := time.ParseInLocation(eventLayout, s.Start, loc)
	if err != nil {
		return Span{}, false
	}
	end, err := time.ParseInLocation(eventLayout, s.End, loc)
	return Span{start, end}, err == nil && end.After(start)
}

func (s Schedule) Validate() error {
	switch s.Kind {
	case Weekly:
		if len(s.Hours) > 7 {
			return errors.New("Each day can have only one set of hours.")
		}
		seen := map[string]bool{}
		for _, h := range s.Hours {
			if len(h.Days) == 0 {
				return errors.New("Pick at least one day for each set of hours.")
			}
			for _, d := range h.Days {
				if !slices.Contains(dayNames[:], d) || seen[d] {
					return errors.New("Each day can have only one set of hours.")
				}
				seen[d] = true
			}
			start, ok1 := minutes(h.Start)
			end, ok2 := minutes(h.End)
			if !ok1 || !ok2 || end <= start {
				return errors.New("Each set of hours needs a start time before its end time.")
			}
		}
		for _, d := range []string{s.From, s.Until} {
			if _, err := time.Parse(dateLayout, d); d != "" && err != nil {
				return errors.New("Enter the season’s dates as full dates.")
			}
		}
		if s.From != "" && s.Until != "" && s.Until < s.From {
			return errors.New("The season must start before it ends.")
		}
	case Event:
		span, ok := s.event(time.UTC)
		if !ok {
			return errors.New("Set when the event starts and ends; it must end after it starts.")
		}
		if span.End.Sub(span.Start) > 31*24*time.Hour {
			return errors.New("A one-off event can run for up to 31 days.")
		}
	default:
		return errors.New("Choose whether the menu repeats weekly or is a one-off event.")
	}
	return nil
}

// Spans is every showing that overlaps from–to, earliest first.
func (s Schedule) Spans(from, to time.Time, loc *time.Location) []Span {
	var out []Span
	switch s.Kind {
	case Event:
		if span, ok := s.event(loc); ok && span.Start.Before(to) && span.End.After(from) {
			out = append(out, span)
		}
	case Weekly:
		f := from.In(loc)
		for day := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc); day.Before(to); day = day.AddDate(0, 0, 1) {
			date := day.Format(dateLayout)
			if s.From != "" && date < s.From || s.Until != "" && date > s.Until {
				continue
			}
			for _, h := range s.Hours {
				if !slices.Contains(h.Days, dayNames[day.Weekday()]) {
					continue
				}
				a, _ := minutes(h.Start)
				b, _ := minutes(h.End)
				span := Span{day.Add(time.Duration(a) * time.Minute), day.Add(time.Duration(b) * time.Minute)}
				if span.End.After(span.Start) && span.Start.Before(to) && span.End.After(from) {
					out = append(out, span)
				}
			}
		}
		slices.SortFunc(out, func(a, b Span) int { return a.Start.Compare(b.Start) })
	}
	return out
}

// Summary is the schedule in words: "Tue – Sat · 7:00 AM – 2:00 PM".
func (s Schedule) Summary(loc *time.Location) string {
	if s.Kind == Event {
		span, ok := s.event(loc)
		if !ok {
			return "Dates not set yet"
		}
		if sameDay(span.Start, span.End) {
			return span.Start.Format("Mon, Jan 2") + " · " + clock(span.Start) + " – " + clock(span.End)
		}
		return span.Start.Format("Mon, Jan 2") + ", " + clock(span.Start) + " – " + span.End.Format("Mon, Jan 2") + ", " + clock(span.End)
	}
	var rows []string
	for _, h := range s.Hours {
		a, _ := time.Parse(clockLayout, h.Start)
		b, _ := time.Parse(clockLayout, h.End)
		rows = append(rows, days(h.Days)+" · "+clock(a)+" – "+clock(b))
	}
	if len(rows) == 0 {
		return "No days set yet"
	}
	text := strings.Join(rows, "; ")
	f, _ := time.Parse(dateLayout, s.From)
	u, _ := time.Parse(dateLayout, s.Until)
	switch {
	case s.From != "" && s.Until != "":
		text += " · " + f.Format("Jan 2") + " – " + u.Format("Jan 2")
	case s.From != "":
		text += " · from " + f.Format("Jan 2")
	case s.Until != "":
		text += " · until " + u.Format("Jan 2")
	}
	return text
}

// days is a set of days in words: "Every day", "Tue – Sat", "Mon, Wed".
func days(names []string) string {
	if len(names) == 7 {
		return "Every day"
	}
	var parts []string
	for i := 0; i < len(Week); {
		if !slices.Contains(names, dayNames[Week[i]]) {
			i++
			continue
		}
		j := i
		for j+1 < len(Week) && slices.Contains(names, dayNames[Week[j+1]]) {
			j++
		}
		switch {
		case j-i >= 2:
			parts = append(parts, dayLabels[Week[i]]+" – "+dayLabels[Week[j]])
		case j > i:
			parts = append(parts, dayLabels[Week[i]], dayLabels[Week[j]])
		default:
			parts = append(parts, dayLabels[Week[i]])
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
}

func clock(t time.Time) string { return t.Format("3:04 PM") }
func sameDay(a, b time.Time) bool {
	return a.Format(dateLayout) == b.Format(dateLayout) || b.Equal(time.Date(a.Year(), a.Month(), a.Day()+1, 0, 0, 0, 0, a.Location()))
}
