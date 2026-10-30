package menu

import (
	"encoding/json"
	"testing"
)

// A menu has one to three sections.
func TestSectionCount(t *testing.T) {
	var m Menu
	if err := json.Unmarshal(Initial, &m.Content); err != nil {
		t.Fatal(err)
	}
	m.Name, m.Schedule = "Regular Menu", Schedule{Kind: Weekly, Hours: []Hours{}}
	all := m.Sections
	for n, ok := range map[int]bool{0: false, 1: true, 2: true, 3: true, 4: false} {
		m.Sections = append([]Section{}, all...)
		for len(m.Sections) < n {
			m.Sections = append(m.Sections, Section{Name: "More", Extras: []string{}, Items: []Item{}})
		}
		m.Sections = m.Sections[:n]
		if err := m.Validate(); (err == nil) != ok {
			t.Errorf("%d sections: %v", n, err)
		}
	}
}
