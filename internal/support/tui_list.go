package support

import (
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

const openFilter = "Status eq 'Open'"

func recentFilter(t time.Time) string {
	return "CreatedDate ge " + t.AddDate(0, 0, -30).Format("2006-01-02")
}

type listScreen struct {
	all       bool // false = open tickets, true = last 30 days
	tickets   []*armsupport.TicketDetails
	loading   bool
	err       error
	search    textinput.Model
	searching bool
	cursor    int
}

type ticketsLoadedMsg struct {
	gen     int
	all     bool
	tickets []*armsupport.TicketDetails
	err     error
}

func newListScreen() *listScreen {
	l := &listScreen{search: newInput()}
	l.search.Prompt = "/ "
	return l
}

func (l *listScreen) init(a *app) tea.Cmd {
	l.err = nil
	if a.client == nil {
		l.loading = false
		return nil
	}
	l.loading = true
	filter := openFilter
	if l.all {
		filter = recentFilter(now())
	}
	c, gen, all := a.client, a.gen, l.all
	return func() tea.Msg {
		t, err := c.ListTickets(a.ctx, filter)
		return ticketsLoadedMsg{gen, all, t, err}
	}
}

func created(t *armsupport.TicketDetails) time.Time {
	if t.Properties != nil && t.Properties.CreatedDate != nil {
		return *t.Properties.CreatedDate
	}
	return time.Time{}
}

func ticketRowText(t *armsupport.TicketDetails) string {
	p := t.Properties
	if p == nil {
		return deref(t.Name)
	}
	sev := ""
	if p.Severity != nil {
		sev = string(*p.Severity)
	}
	return created(t).Format("2006-01-02 15:04") + "  " + sev + "  " + deref(p.Status) + "  " + deref(p.Title) + "  " + deref(p.SupportTicketID)
}

// visible returns the tickets matching the search, in display order.
func (l *listScreen) visible() []*armsupport.TicketDetails {
	texts := make([]string, len(l.tickets))
	for i, t := range l.tickets {
		texts[i] = ticketRowText(t)
	}
	var out []*armsupport.TicketDetails
	for _, i := range rank(l.search.Value(), texts) {
		out = append(out, l.tickets[i])
	}
	return out
}

func (l *listScreen) update(a *app, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case ticketsLoadedMsg:
		if m.gen != a.gen || m.all != l.all {
			return nil
		}
		l.loading, l.err, l.tickets, l.cursor = false, m.err, m.tickets, 0
		sort.SliceStable(l.tickets, func(i, j int) bool { return created(l.tickets[i]).After(created(l.tickets[j])) })
	case tea.KeyPressMsg:
		return l.key(a, m)
	case tea.PasteMsg:
		if l.searching {
			var cmd tea.Cmd
			l.search, cmd = l.search.Update(m)
			l.cursor = 0
			return cmd
		}
	}
	return nil
}

func (l *listScreen) key(a *app, m tea.KeyPressMsg) tea.Cmd {
	k := m.String()
	switch k {
	case "up":
		l.cursor = max(l.cursor-1, 0)
		return nil
	case "down":
		l.cursor = max(min(l.cursor+1, len(l.visible())-1), 0)
		return nil
	case "enter":
		if rows := l.visible(); l.cursor < len(rows) {
			return a.push(newDetailScreen(deref(rows[l.cursor].Name)))
		}
		return nil
	case "esc":
		l.search.SetValue("")
		l.search.Blur()
		l.searching, l.cursor = false, 0
		return nil
	}
	if l.searching {
		before := l.search.Value()
		var cmd tea.Cmd
		l.search, cmd = l.search.Update(m)
		if l.search.Value() != before {
			l.cursor = 0
		}
		return cmd
	}
	switch k {
	case "/":
		l.searching = true
		return l.search.Focus()
	case "f":
		l.all = !l.all
		return l.init(a)
	case "ctrl+r":
		return l.init(a)
	case "n":
		return a.push(newCreateScreen())
	case "t":
		p, err := a.d.loadProfile()
		if err != nil {
			l.err = err
			return nil
		}
		return a.push(newScopeScreen(p))
	case "q":
		a.quitting = true
		return tea.Quit
	}
	return nil
}

func (l *listScreen) view(a *app) string {
	var b strings.Builder
	if l.all {
		b.WriteString("Tickets: all, last 30 days")
	} else {
		b.WriteString("Tickets: open")
	}
	if l.searching || l.search.Value() != "" {
		b.WriteString("\n" + l.search.View())
	}
	switch {
	case l.loading:
		b.WriteString("\nLoading…")
	case l.err != nil:
		b.WriteString("\n" + errView(l.err))
	default:
		rows := l.visible()
		if len(rows) == 0 {
			b.WriteString("\nNo tickets.")
		}
		n := a.bodyHeight()
		start := max(min(l.cursor-n/2, len(rows)-n), 0)
		for i := start; i < len(rows) && i < start+n; i++ {
			mark := "  "
			if i == l.cursor {
				mark = "> "
			}
			b.WriteString("\n" + mark + ticketRowText(rows[i]))
		}
	}
	b.WriteString("\n\nn new · f open/all · / search · t scope · ctrl+r reload · q quit")
	return b.String()
}
