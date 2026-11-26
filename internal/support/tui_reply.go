package support

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

type replyScreen struct {
	ticket  armsupport.TicketDetails
	subject textinput.Model
	body    textarea.Model
	attach  textinput.Model
	focus   int // 0 subject, 1 body, 2 attachment path
	sending bool
	err     error
}

type replySentMsg struct {
	gen int
	err error
}

// newTextarea makes a textarea with cursor blinking off.
func newTextarea() textarea.Model {
	m := textarea.New()
	m.ShowLineNumbers = false
	st := m.Styles()
	st.Cursor.Blink = false
	m.SetStyles(st)
	return m
}

func newReplyScreen(t armsupport.TicketDetails) *replyScreen {
	r := &replyScreen{ticket: t, subject: newInput(), body: newTextarea(), attach: newInput()}
	r.subject.Prompt, r.attach.Prompt = "Subject: ", "Attachment path (optional): "
	if t.Properties != nil {
		r.subject.SetValue(deref(t.Properties.Title))
	}
	r.body.SetHeight(8)
	r.setFocus(0)
	return r
}

func (r *replyScreen) setFocus(i int) tea.Cmd {
	r.focus = (i + 3) % 3
	r.subject.Blur()
	r.body.Blur()
	r.attach.Blur()
	switch r.focus {
	case 0:
		return r.subject.Focus()
	case 1:
		return r.body.Focus()
	}
	return r.attach.Focus()
}

func (r *replyScreen) init(a *app) tea.Cmd { return nil }

func (r *replyScreen) update(a *app, msg tea.Msg) tea.Cmd {
	if m, ok := msg.(replySentMsg); ok {
		if m.gen != a.gen {
			return nil
		}
		r.sending, r.err = false, m.err
		if m.err != nil {
			return nil
		}
		a.pop()
		return a.top().init(a)
	}
	if r.sending {
		return nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc":
			a.pop()
			return nil
		case "tab":
			return r.setFocus(r.focus + 1)
		case "shift+tab":
			return r.setFocus(r.focus - 1)
		case "ctrl+s":
			return r.send(a)
		}
	}
	var cmd tea.Cmd
	switch r.focus {
	case 0:
		r.subject, cmd = r.subject.Update(msg)
	case 1:
		r.body, cmd = r.body.Update(msg)
	default:
		r.attach, cmd = r.attach.Update(msg)
	}
	return cmd
}

func (r *replyScreen) send(a *app) tea.Cmd {
	body := strings.TrimSpace(r.body.Value())
	if body == "" {
		r.err = errors.New("Message body is required")
		return nil
	}
	subject := r.subject.Value()
	ticket := deref(r.ticket.Name)
	path := strings.TrimSpace(r.attach.Value())
	ws := ""
	if r.ticket.Properties != nil {
		ws = deref(r.ticket.Properties.FileWorkspaceName)
	}
	var u *upload
	if path != "" {
		if ws == "" {
			r.err = errors.New(errNoWorkspace)
			return nil
		}
		up, err := readUpload(path)
		if err != nil {
			r.err = err
			return nil
		}
		u = &up
	}
	r.err, r.sending = nil, true
	c, gen, name := a.client, a.gen, newName()
	return func() tea.Msg {
		if err := c.CheckCommunicationName(a.ctx, ticket, name); err != nil {
			return replySentMsg{gen, err}
		}
		if u != nil {
			if err := uploadFile(a.ctx, c, ws, *u); err != nil {
				return replySentMsg{gen, err}
			}
		}
		_, err := c.CreateCommunication(a.ctx, ticket, name, armsupport.CommunicationDetails{
			Properties: &armsupport.CommunicationDetailsProperties{Subject: &subject, Body: &body},
		})
		return replySentMsg{gen, err}
	}
}

func (r *replyScreen) view(a *app) string {
	r.body.SetWidth(max(a.width-2, 20))
	var b strings.Builder
	b.WriteString("Reply to " + deref(r.ticket.Name) + "\n" + r.subject.View() + "\n" + r.body.View() + "\n" + r.attach.View())
	if r.err != nil {
		b.WriteString("\n" + errView(r.err))
	}
	if r.sending {
		b.WriteString("\nSending…")
	}
	b.WriteString("\n\ntab next field · ctrl+s send · esc cancel")
	return b.String()
}
