package support

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// newInput makes a textinput with cursor blinking off (a blinking cursor re-arms a timer Cmd forever).
func newInput() textinput.Model {
	m := textinput.New()
	st := m.Styles()
	st.Cursor.Blink = false
	m.SetStyles(st)
	return m
}

// picker is a filterable single-select list.
type picker struct {
	input  textinput.Model
	items  []string
	shown  []int // indices into items, best match first
	cursor int
}

func newPicker(items []string, query string) picker {
	p := picker{input: newInput()}
	p.input.Prompt = "/ "
	p.input.SetValue(query)
	p.input.Focus()
	p.setItems(items)
	return p
}

func (p *picker) setItems(items []string) {
	p.items = items
	p.refilter()
}

func (p *picker) refilter() {
	p.shown = rank(p.input.Value(), p.items)
	p.cursor = 0
}

// update returns the picked item index when enter is pressed on a row.
func (p *picker) update(msg tea.Msg) (picked int, done bool, cmd tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "up":
			p.cursor = max(p.cursor-1, 0)
			return 0, false, nil
		case "down":
			p.cursor = max(min(p.cursor+1, len(p.shown)-1), 0)
			return 0, false, nil
		case "enter":
			if p.cursor < len(p.shown) {
				return p.shown[p.cursor], true, nil
			}
			return 0, false, nil
		}
	}
	before := p.input.Value()
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.refilter()
	}
	return 0, false, cmd
}

// view renders the filter line and a window of at most height rows around the cursor.
func (p picker) view(height int) string {
	rows := max(height-1, 1)
	start := max(min(p.cursor-rows/2, len(p.shown)-rows), 0)
	var b strings.Builder
	b.WriteString(p.input.View())
	for i := start; i < len(p.shown) && i < start+rows; i++ {
		mark := "  "
		if i == p.cursor {
			mark = "> "
		}
		b.WriteString("\n" + mark + p.items[p.shown[i]])
	}
	return b.String()
}
