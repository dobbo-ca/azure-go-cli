package support

import "testing"

func TestPickerFilters(t *testing.T) {
	p := newPicker([]string{"a", "Pacific Standard Time", "Mountain Standard Time"}, "")
	for _, m := range keys("pacific") {
		p.update(m)
	}
	if len(p.shown) != 1 || p.shown[0] != 1 {
		t.Fatalf("shown = %v", p.shown)
	}
	got, done, _ := p.update(keys("enter")[0])
	if !done || got != 1 {
		t.Fatalf("picked %d done %v", got, done)
	}
}

func TestPickerNoMatchDownEnter(t *testing.T) {
	p := newPicker([]string{"a", "b"}, "")
	for _, m := range keys("zzzz", "down") {
		p.update(m)
	}
	if _, done, _ := p.update(keys("enter")[0]); done {
		t.Fatal("picked with no matches")
	}
}
