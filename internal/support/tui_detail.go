package support

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

type detailMode int

const (
	modeView detailMode = iota
	modeConfirmStatus
	modeSeverity
	modePhone
	modeAttach
)

const errNoWorkspace = "this ticket has no file workspace"

type detailScreen struct {
	name    string
	ticket  *armsupport.TicketDetails
	comms   []*armsupport.CommunicationDetails
	vp      viewport.Model
	mode    detailMode
	input   textinput.Model
	sev     picker
	newSev  armsupport.SeverityLevel // chosen severity while the phone prompt is open
	status  string
	err     error
	loading bool
}

type ticketLoadedMsg struct {
	gen    int
	name   string
	ticket *armsupport.TicketDetails
	comms  []*armsupport.CommunicationDetails
	err    error
}

// ticketChangedMsg reports a finished status, severity or attach action.
type ticketChangedMsg struct {
	gen    int
	name   string
	status string
	err    error
}

var severities = []string{
	string(armsupport.SeverityLevelMinimal), string(armsupport.SeverityLevelModerate),
	string(armsupport.SeverityLevelCritical), string(armsupport.SeverityLevelHighestcriticalimpact),
}

func newDetailScreen(name string) *detailScreen {
	vp := viewport.New()
	vp.SoftWrap = true
	return &detailScreen{name: name, input: newInput(), vp: vp}
}

func (d *detailScreen) init(a *app) tea.Cmd {
	d.loading = true
	c, gen, name := a.client, a.gen, d.name
	return func() tea.Msg {
		t, err := c.GetTicket(a.ctx, name)
		if err != nil {
			return ticketLoadedMsg{gen: gen, name: name, err: err}
		}
		comms, err := c.ListCommunications(a.ctx, name, "")
		return ticketLoadedMsg{gen, name, &t, comms, err}
	}
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// renderThread is the ticket header, its description and the communications, oldest first.
func renderThread(t armsupport.TicketDetails, comms []*armsupport.CommunicationDetails) string {
	p := t.Properties
	if p == nil {
		p = &armsupport.TicketDetailsProperties{}
	}
	sev := ""
	if p.Severity != nil {
		sev = string(*p.Severity)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s · %s · %s\n%s › %s\nCreated %s · Modified %s · %s\n\n%s\n",
		deref(p.Title), deref(p.SupportTicketID), deref(p.Status), sev,
		deref(p.ServiceDisplayName), deref(p.ProblemClassificationDisplayName),
		fmtTime(p.CreatedDate), fmtTime(p.ModifiedDate), deref(p.SupportPlanDisplayName),
		htmlToText(deref(p.Description)))
	sorted := append([]*armsupport.CommunicationDetails(nil), comms...)
	at := func(c *armsupport.CommunicationDetails) time.Time {
		if c.Properties != nil && c.Properties.CreatedDate != nil {
			return *c.Properties.CreatedDate
		}
		return time.Time{}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return at(sorted[i]).Before(at(sorted[j])) })
	for _, c := range sorted {
		cp := c.Properties
		if cp == nil {
			continue
		}
		who := "from you"
		if cp.CommunicationDirection != nil && *cp.CommunicationDirection == armsupport.CommunicationDirectionInbound {
			who = "from Microsoft"
		}
		fmt.Fprintf(&b, "\n── %s · %s · %s ──\n%s\n%s\n", who, deref(cp.Sender), fmtTime(cp.CreatedDate), deref(cp.Subject), htmlToText(deref(cp.Body)))
	}
	return b.String()
}

func (d *detailScreen) props() *armsupport.TicketDetailsProperties {
	if d.ticket == nil || d.ticket.Properties == nil {
		return &armsupport.TicketDetailsProperties{}
	}
	return d.ticket.Properties
}

// change runs fn off the UI goroutine and reloads the ticket when it succeeds.
// fn gets the client of the scope the action was started in.
func (d *detailScreen) change(a *app, fn func(c Client) (status string, err error)) tea.Cmd {
	gen, name, c := a.gen, d.name, a.client
	return func() tea.Msg {
		status, err := fn(c)
		return ticketChangedMsg{gen, name, status, err}
	}
}

func (d *detailScreen) update(a *app, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case ticketLoadedMsg:
		if m.gen != a.gen || m.name != d.name {
			return nil
		}
		d.loading, d.err = false, m.err
		if m.err == nil {
			d.ticket, d.comms = m.ticket, m.comms
			d.vp.SetContent(renderThread(*m.ticket, m.comms))
		}
		return nil
	case ticketChangedMsg:
		if m.gen != a.gen || m.name != d.name {
			return nil
		}
		d.err, d.status = m.err, m.status
		if m.err != nil {
			return nil
		}
		return d.init(a)
	case tea.WindowSizeMsg:
		d.vp.SetWidth(m.Width)
		d.vp.SetHeight(a.bodyHeight())
	case tea.KeyPressMsg:
		return d.key(a, m)
	case tea.PasteMsg:
		if d.mode == modeAttach || d.mode == modePhone {
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(m)
			return cmd
		}
	}
	return nil
}

func (d *detailScreen) key(a *app, m tea.KeyPressMsg) tea.Cmd {
	k := m.String()
	if k == "ctrl+r" {
		d.mode = modeView
		return d.init(a)
	}
	if k == "esc" {
		if d.mode == modeView {
			a.pop()
		}
		d.mode = modeView
		return nil
	}
	switch d.mode {
	case modeConfirmStatus:
		switch k {
		case "y":
			d.mode = modeView
			next := armsupport.StatusClosed
			if strings.EqualFold(deref(d.props().Status), "closed") {
				next = armsupport.StatusOpen
			}
			return d.change(a, func(c Client) (string, error) {
				_, err := c.UpdateTicket(a.ctx, d.name, armsupport.UpdateSupportTicket{Status: &next})
				return "", err
			})
		case "n":
			d.mode = modeView
		}
		return nil
	case modeSeverity:
		i, done, cmd := d.sev.update(m)
		if !done {
			return cmd
		}
		sev := armsupport.SeverityLevel(severities[i])
		cp := d.props().ContactDetails
		method := armsupport.PreferredContactMethodEmail
		phone := ""
		if cp != nil {
			phone = deref(cp.PhoneNumber)
			if cp.PreferredContactMethod != nil {
				method = *cp.PreferredContactMethod
			}
		}
		if needsPhone(method, sev) && phone == "" {
			d.mode, d.newSev = modePhone, sev
			d.input.SetValue("")
			d.input.Prompt = "Phone number: "
			return d.input.Focus()
		}
		d.mode = modeView
		return d.setSeverity(a, sev, "")
	case modePhone:
		if k == "enter" {
			phone := strings.TrimSpace(d.input.Value())
			if phone == "" {
				d.err = fmt.Errorf("a phone number is required for severity %s", d.newSev)
				return nil
			}
			d.mode, d.err = modeView, nil
			return d.setSeverity(a, d.newSev, phone)
		}
	case modeAttach:
		if k == "enter" {
			path := strings.TrimSpace(d.input.Value())
			ws := deref(d.props().FileWorkspaceName)
			d.mode = modeView
			return d.change(a, func(c Client) (string, error) {
				u, err := readUpload(path)
				if err != nil {
					return "", err
				}
				if err := uploadFile(a.ctx, c, ws, u); err != nil {
					return "", err
				}
				return "Uploaded " + u.Name, nil
			})
		}
	default:
		if d.ticket == nil && (k == "s" || k == "v" || k == "a") {
			return nil // the actions need the loaded ticket
		}
		switch k {
		case "s":
			d.err, d.mode = nil, modeConfirmStatus
			return nil
		case "v":
			d.err, d.mode = nil, modeSeverity
			d.sev = newPicker(severities, "")
			return nil
		case "r":
			if d.ticket != nil {
				return a.push(newReplyScreen(*d.ticket))
			}
			return nil
		case "a":
			d.err = nil
			if deref(d.props().FileWorkspaceName) == "" {
				d.err = errors.New(errNoWorkspace)
				return nil
			}
			d.mode = modeAttach
			d.input.SetValue("")
			d.input.Prompt = "File path: "
			return d.input.Focus()
		}
		var cmd tea.Cmd
		d.vp, cmd = d.vp.Update(m)
		return cmd
	}
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(m)
	return cmd
}

func (d *detailScreen) setSeverity(a *app, sev armsupport.SeverityLevel, phone string) tea.Cmd {
	u := armsupport.UpdateSupportTicket{Severity: &sev}
	if phone != "" {
		u.ContactDetails = &armsupport.UpdateContactProfile{PhoneNumber: &phone}
	}
	return d.change(a, func(c Client) (string, error) {
		_, err := c.UpdateTicket(a.ctx, d.name, u)
		return "", err
	})
}

func (d *detailScreen) view(a *app) string {
	var b strings.Builder
	if d.loading && d.ticket == nil {
		b.WriteString("Loading…")
	} else {
		d.vp.SetWidth(max(a.width, 20))
		d.vp.SetHeight(a.bodyHeight())
		b.WriteString(d.vp.View())
	}
	if d.err != nil {
		b.WriteString("\n" + errView(d.err))
	}
	if d.status != "" {
		b.WriteString("\n" + d.status)
	}
	switch d.mode {
	case modeConfirmStatus:
		verb := "Close"
		if strings.EqualFold(deref(d.props().Status), "closed") {
			verb = "Reopen"
		}
		b.WriteString("\n" + verb + " this ticket? (y/n)")
	case modeSeverity:
		b.WriteString("\n" + d.sev.view(6))
	case modePhone, modeAttach:
		b.WriteString("\n" + d.input.View())
	default:
		b.WriteString("\n\nr reply · a attach · s close/reopen · v severity · ctrl+r reload · esc back")
	}
	return b.String()
}
