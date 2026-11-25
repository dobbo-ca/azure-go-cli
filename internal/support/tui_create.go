package support

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

type createStep int

const (
	stepDescribe createStep = iota
	stepService
	stepResource
	stepSeverity
	stepContact
	stepAttach
	stepReview
)

var stepNames = []string{"Describe", "Service", "Resource", "Severity", "Contact", "Attach", "Review"}

const quotaHint = "Quota requests are not filed here. Use `az quota` or `az support in-subscription tickets create --quota-change-*`."

type createScreen struct {
	step    createStep
	title   textinput.Model
	desc    textarea.Model
	start   textinput.Model // empty = now
	field   int             // focused field on the describe step
	svc     picker
	svcInit bool
	row     *catalogRow
	hint    string
	loading bool // catalog load pending

	resources  []resourceRef
	resPick    picker
	resLoading bool
	resourceID string
	resInput   textinput.Model
	resManual  bool

	sev        int  // index into severities
	require24  bool // only sent for critical and above
	noConsent  bool // advanced diagnostic consent is Yes unless toggled
	first      textinput.Model
	last       textinput.Model
	email      textinput.Model
	country    string // ISO 3166-1 alpha-3
	zone       string // Windows time zone name
	lang       string
	phoneMeth  bool // contact method phone instead of email
	phone      textinput.Model
	extra      textinput.Model // additional emails, comma-separated
	cfield     int             // index into contactFields()
	cpick      int             // 0 none, else the contactField id of the open picker
	cpicker    picker
	meInit     bool
	meNote     string
	attach     textinput.Model
	attachU    *upload
	submitting bool

	err error
}

type meLoadedMsg struct {
	m   me
	err error
}

type ticketCreatedMsg struct {
	gen  int
	name string
	err  error
}

type catalogLoadedMsg struct {
	rows []catalogRow
	err  error
}

type resourcesLoadedMsg struct {
	gen  int
	svc  string
	rows []resourceRef
	err  error
}

func newCreateScreen() *createScreen {
	c := &createScreen{title: newInput(), desc: newTextarea(), start: newInput(), resInput: newInput(),
		require24: true, first: newInput(), last: newInput(), email: newInput(), phone: newInput(), extra: newInput(), attach: newInput()}
	c.first.Prompt, c.last.Prompt, c.email.Prompt = "First name: ", "Last name: ", "Email: "
	c.phone.Prompt, c.extra.Prompt = "Phone number: ", "Additional emails (comma-separated): "
	c.attach.Prompt = "Attachment path (optional): "
	c.title.Prompt = "Title: "
	c.start.Prompt = "Problem start time (optional, UTC, empty = now): "
	c.resInput.Prompt = "Resource ID: "
	c.desc.SetHeight(6)
	c.setField(0)
	return c
}

func (c *createScreen) setField(i int) tea.Cmd {
	c.field = (i + 3) % 3
	c.title.Blur()
	c.desc.Blur()
	c.start.Blur()
	switch c.field {
	case 0:
		return c.title.Focus()
	case 1:
		return c.desc.Focus()
	}
	return c.start.Focus()
}

func (c *createScreen) init(a *app) tea.Cmd {
	if a.catalog != nil {
		return nil
	}
	c.loading = true
	cl, ctx := a.client, a.ctx
	return func() tea.Msg {
		rows, err := loadCatalog(ctx, cl, 8)
		return catalogLoadedMsg{rows, err}
	}
}

func (c *createScreen) needsResourceStep(a *app) bool {
	return c.row != nil && isTechnicalService(c.row.ServiceName) && !a.scope.TenantLevel() && len(c.row.ResourceTypes) > 0
}

func catalogItems(rows []catalogRow) []string {
	items := make([]string, len(rows))
	for i, r := range rows {
		items[i] = r.text()
	}
	return items
}

func (c *createScreen) update(a *app, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case catalogLoadedMsg:
		c.loading = false
		a.catalog, a.catErr = m.rows, m.err
		if c.svcInit {
			c.svc.setItems(catalogItems(m.rows))
		}
		return nil
	case resourcesLoadedMsg:
		if m.gen != a.gen || c.step != stepResource || m.svc != c.row.ServiceName {
			return nil
		}
		c.resLoading, c.err, c.resources = false, m.err, m.rows
		c.resPick.setItems(c.resourceItems())
		return nil
	case meLoadedMsg:
		c.fillContact(a, m)
		return nil
	case ticketCreatedMsg:
		if m.gen != a.gen {
			return nil
		}
		c.submitting = false
		if m.err != nil {
			c.err = m.err
			return nil
		}
		return a.replace(newDetailScreen(m.name))
	case tea.KeyPressMsg:
		if c.submitting {
			return nil
		}
		switch m.String() {
		case "ctrl+n":
			return c.next(a)
		case "esc":
			return c.back(a)
		}
	}
	switch c.step {
	case stepDescribe:
		return c.updateDescribe(a, msg)
	case stepService:
		return c.updateService(a, msg)
	case stepResource:
		return c.updateResource(a, msg)
	case stepSeverity:
		return c.updateSeverity(a, msg)
	case stepContact:
		return c.updateContact(a, msg)
	case stepAttach:
		var cmd tea.Cmd
		c.attach, cmd = c.attach.Update(msg)
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
			return c.next(a)
		}
		return cmd
	case stepReview:
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
			return c.submit(a)
		}
	}
	return nil
}

func (c *createScreen) critical() bool { return c.sev >= 2 }

func (c *createScreen) updateSeverity(a *app, msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch k.String() {
	case "enter":
		return c.next(a)
	case "up":
		c.sev = max(c.sev-1, 0)
	case "down":
		c.sev = min(c.sev+1, len(severities)-1)
	case "space":
		if c.critical() {
			c.require24 = !c.require24
		}
	case "c":
		c.noConsent = !c.noConsent
	}
	return nil
}

// contact fields, in tab order; phone only when needed.
const (
	cfFirst = iota + 1
	cfLast
	cfEmail
	cfCountry
	cfZone
	cfLang
	cfMethod
	cfPhone
	cfExtra
)

func (c *createScreen) method() armsupport.PreferredContactMethod {
	if c.phoneMeth {
		return armsupport.PreferredContactMethodPhone
	}
	return armsupport.PreferredContactMethodEmail
}

func (c *createScreen) sevLevel() armsupport.SeverityLevel {
	return armsupport.SeverityLevel(severities[c.sev])
}

func (c *createScreen) contactFields() []int {
	f := []int{cfFirst, cfLast, cfEmail, cfCountry, cfZone, cfLang, cfMethod}
	if needsPhone(c.method(), c.sevLevel()) {
		f = append(f, cfPhone)
	}
	return append(f, cfExtra)
}

func (c *createScreen) textInput(id int) *textinput.Model {
	switch id {
	case cfFirst:
		return &c.first
	case cfLast:
		return &c.last
	case cfEmail:
		return &c.email
	case cfPhone:
		return &c.phone
	case cfExtra:
		return &c.extra
	}
	return nil
}

func (c *createScreen) setContactFocus(i int) tea.Cmd {
	f := c.contactFields()
	c.cfield = (i + len(f)) % len(f)
	for _, id := range f {
		if t := c.textInput(id); t != nil {
			t.Blur()
		}
	}
	if t := c.textInput(f[c.cfield]); t != nil {
		return t.Focus()
	}
	return nil
}

func (c *createScreen) fillContact(a *app, m meLoadedMsg) {
	if m.err != nil {
		c.meNote = "Could not read your profile from Microsoft Graph: " + m.err.Error()
	}
	d := contactFromMe(m.m, a.d.getenv, a.d.localZone())
	for _, f := range []struct {
		in *textinput.Model
		v  string
	}{{&c.first, d.FirstName}, {&c.last, d.LastName}, {&c.email, d.Email}} {
		if f.in.Value() == "" {
			f.in.SetValue(f.v)
		}
	}
	for _, f := range []struct {
		p *string
		v string
	}{{&c.country, d.Country}, {&c.zone, d.TimeZone}, {&c.lang, d.Language}} {
		if *f.p == "" {
			*f.p = f.v
		}
	}
}

func (c *createScreen) pickerItems(id int) []string {
	switch id {
	case cfCountry:
		return countryOptions()
	case cfZone:
		return windowsZoneNames()
	}
	return supportLanguages
}

func (c *createScreen) updateContact(a *app, msg tea.Msg) tea.Cmd {
	if c.cpick != 0 {
		i, done, cmd := c.cpicker.update(msg)
		if !done {
			return cmd
		}
		v := c.cpicker.items[i]
		switch c.cpick {
		case cfCountry:
			v = v[strings.LastIndex(v, "(")+1 : len(v)-1]
			c.country = v
		case cfZone:
			c.zone = v
		default:
			c.lang = v
		}
		c.cpick = 0
		return nil
	}
	f := c.contactFields()
	id := f[c.cfield]
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "tab":
			return c.setContactFocus(c.cfield + 1)
		case "shift+tab":
			return c.setContactFocus(c.cfield - 1)
		case "enter":
			if id == cfCountry || id == cfZone || id == cfLang {
				c.cpick, c.cpicker = id, newPicker(c.pickerItems(id), "")
				return nil
			}
			return c.next(a)
		case "space":
			if id == cfMethod {
				c.phoneMeth = !c.phoneMeth
				return nil
			}
		}
	}
	if t := c.textInput(id); t != nil {
		var cmd tea.Cmd
		*t, cmd = t.Update(msg)
		return cmd
	}
	return nil
}

func (c *createScreen) contactError() error {
	for _, r := range []struct{ label, v string }{
		{"First name", c.first.Value()}, {"Last name", c.last.Value()}, {"Email", c.email.Value()},
		{"Country", c.country}, {"Time zone", c.zone}, {"Language", c.lang},
	} {
		if strings.TrimSpace(r.v) == "" {
			return errors.New(r.label + " is required")
		}
	}
	if needsPhone(c.method(), c.sevLevel()) && strings.TrimSpace(c.phone.Value()) == "" {
		return errors.New("Phone number is required")
	}
	return nil
}

// ticketDetails builds the create request from the screen state.
func (c *createScreen) ticketDetails(a *app) armsupport.TicketDetails {
	sev, method := c.sevLevel(), c.method()
	consent := armsupport.ConsentYes
	if c.noConsent {
		consent = armsupport.ConsentNo
	}
	var emails []string
	for _, e := range strings.Split(c.extra.Value(), ",") {
		if e = strings.TrimSpace(e); e != "" {
			emails = append(emails, e)
		}
	}
	cp := &armsupport.ContactProfile{
		Country: ref(c.country), FirstName: ref(strings.TrimSpace(c.first.Value())), LastName: ref(strings.TrimSpace(c.last.Value())),
		PreferredContactMethod: &method, PreferredSupportLanguage: ref(c.lang), PreferredTimeZone: ref(c.zone),
		PrimaryEmailAddress: ref(strings.TrimSpace(c.email.Value())), AdditionalEmailAddresses: strPtrs(emails),
	}
	if p := strings.TrimSpace(c.phone.Value()); p != "" {
		cp.PhoneNumber = &p
	}
	p := &armsupport.TicketDetailsProperties{
		Title: ref(strings.TrimSpace(c.title.Value())), Description: ref(strings.TrimSpace(c.desc.Value())),
		ProblemClassificationID: ref(c.row.ProblemID), Severity: &sev, AdvancedDiagnosticConsent: &consent, ContactDetails: cp,
	}
	if c.critical() && c.require24 {
		p.Require24X7Response = ref(true)
	}
	if s := strings.TrimSpace(c.start.Value()); s != "" {
		if t, err := parseStartTime(s); err == nil {
			p.ProblemStartTime = &t
		}
	}
	if c.resourceID != "" {
		p.TechnicalTicketDetails = &armsupport.TechnicalTicketDetails{ResourceID: &c.resourceID}
	}
	return armsupport.TicketDetails{Properties: p}
}

func (c *createScreen) submit(a *app) tea.Cmd {
	c.err, c.submitting = nil, true
	cl, gen, name, td, up := a.client, a.gen, newName(), c.ticketDetails(a), c.attachU
	return func() tea.Msg {
		_, err := createTicket(a.ctx, cl, name, td, up)
		return ticketCreatedMsg{gen, name, err}
	}
}

func (c *createScreen) updateDescribe(a *app, msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "tab":
			return c.setField(c.field + 1)
		case "shift+tab":
			return c.setField(c.field - 1)
		case "enter":
			if c.field != 1 {
				return c.next(a)
			}
		}
	}
	var cmd tea.Cmd
	switch c.field {
	case 0:
		c.title, cmd = c.title.Update(msg)
	case 1:
		c.desc, cmd = c.desc.Update(msg)
	default:
		c.start, cmd = c.start.Update(msg)
	}
	return cmd
}

func (c *createScreen) updateService(a *app, msg tea.Msg) tea.Cmd {
	i, done, cmd := c.svc.update(msg)
	if !done {
		return cmd
	}
	return c.pickService(a, a.catalog[i])
}

func (c *createScreen) pickService(a *app, r catalogRow) tea.Cmd {
	if isQuotaService(r.ServiceName) {
		c.hint = quotaHint
		return nil
	}
	c.hint, c.row, c.resourceID = "", &r, ""
	return c.enter(a, stepResource)
}

func (c *createScreen) resourceItems() []string {
	items := []string{"Skip", "Enter resource ID…"}
	for _, r := range c.resources {
		items = append(items, r.Name+" ("+r.Type+")")
	}
	return items
}

func (c *createScreen) updateResource(a *app, msg tea.Msg) tea.Cmd {
	if c.resManual {
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" {
			return c.next(a)
		}
		var cmd tea.Cmd
		c.resInput, cmd = c.resInput.Update(msg)
		return cmd
	}
	i, done, cmd := c.resPick.update(msg)
	if !done {
		return cmd
	}
	switch i {
	case 0:
		c.resourceID = ""
		return c.enter(a, stepSeverity)
	case 1:
		c.resManual = true
		c.resInput.SetValue("")
		return c.resInput.Focus()
	}
	c.resourceID = c.resources[i-2].ID
	return c.enter(a, stepSeverity)
}

// next validates the current step and moves on.
func (c *createScreen) next(a *app) tea.Cmd {
	c.err = nil
	switch c.step {
	case stepDescribe:
		switch {
		case strings.TrimSpace(c.title.Value()) == "":
			c.err = errors.New("Title is required")
		case strings.TrimSpace(c.desc.Value()) == "":
			c.err = errors.New("Description is required")
		}
		if c.err == nil && strings.TrimSpace(c.start.Value()) != "" {
			_, c.err = parseStartTime(strings.TrimSpace(c.start.Value()))
		}
		if c.err != nil {
			return nil
		}
		return c.enter(a, stepService)
	case stepService:
		// same as enter on the highlighted row
		return c.updateService(a, tea.KeyPressMsg{Code: tea.KeyEnter})
	case stepResource:
		if c.resManual {
			id := strings.TrimSpace(c.resInput.Value())
			if c.err = validateTechnicalResource(id, a.scope.SubscriptionID); c.err != nil {
				return nil
			}
			c.resourceID = id
			return c.enter(a, stepSeverity)
		}
		return c.updateResource(a, tea.KeyPressMsg{Code: tea.KeyEnter})
	case stepSeverity:
		return c.enter(a, stepContact)
	case stepContact:
		if c.cpick != 0 {
			return c.updateContact(a, tea.KeyPressMsg{Code: tea.KeyEnter})
		}
		if c.err = c.contactError(); c.err != nil {
			return nil
		}
		return c.enter(a, stepAttach)
	case stepAttach:
		c.attachU = nil
		if p := strings.TrimSpace(c.attach.Value()); p != "" {
			u, err := readUpload(p)
			if c.err = err; err != nil {
				return nil
			}
			c.attachU = &u
		}
		return c.enter(a, stepReview)
	}
	return nil
}

// enter makes step the current step and starts whatever it needs. Asking for
// the resource step lands on severity when that step does not apply.
func (c *createScreen) enter(a *app, step createStep) tea.Cmd {
	if step == stepResource && !c.needsResourceStep(a) {
		step = stepSeverity
	}
	c.step = step
	switch step {
	case stepContact:
		c.cpick = 0
		cmd := c.setContactFocus(0)
		if c.meInit {
			return cmd
		}
		c.meInit = true
		s := a.scope
		fetch := a.d.fetchMe
		return tea.Batch(cmd, func() tea.Msg {
			m, err := fetch(a.ctx, s)
			return meLoadedMsg{m, err}
		})
	case stepAttach:
		return c.attach.Focus()
	case stepService:
		if !c.svcInit {
			c.svcInit = true
			c.svc = newPicker(catalogItems(a.catalog), queryFromTitle(c.title.Value()))
		}
	case stepResource:
		c.resManual, c.resources, c.resLoading = false, nil, true
		c.resPick = newPicker(c.resourceItems(), "")
		s, types, gen, svc := a.scope, c.row.ResourceTypes, a.gen, c.row.ServiceName
		list := a.d.listResources
		return func() tea.Msg {
			rows, err := list(a.ctx, s, types)
			return resourcesLoadedMsg{gen, svc, rows, err}
		}
	}
	return nil
}

// back goes one step back, keeping every value; from the first step it leaves the screen.
func (c *createScreen) back(a *app) tea.Cmd {
	c.err = nil
	switch c.step {
	case stepDescribe:
		a.pop()
	case stepResource:
		if c.resManual {
			c.resManual = false
			return nil
		}
		c.step = stepService
	case stepSeverity:
		if c.needsResourceStep(a) {
			c.step = stepResource
		} else {
			c.step = stepService
		}
	case stepContact:
		if c.cpick != 0 {
			c.cpick = 0
			return nil
		}
		c.step--
	default:
		c.step--
	}
	return nil
}

func (c *createScreen) view(a *app) string {
	c.desc.SetWidth(max(a.width-2, 20))
	var b strings.Builder
	fmt.Fprintf(&b, "New ticket · step %d/%d · %s\n\n", c.step+1, len(stepNames), stepNames[c.step])
	switch c.step {
	case stepDescribe:
		b.WriteString(c.title.View() + "\n" + c.desc.View() + "\n" + c.start.View())
	case stepService:
		if c.loading {
			b.WriteString("Loading services…\n")
		}
		if a.catErr != nil {
			b.WriteString(errView(a.catErr) + "\n")
		}
		if c.hint != "" {
			b.WriteString(c.hint + "\n")
		}
		b.WriteString(c.svc.view(a.bodyHeight() - 4))
	case stepResource:
		if c.resManual {
			b.WriteString(c.resInput.View())
		} else {
			if c.resLoading {
				b.WriteString("Loading resources…\n")
			}
			b.WriteString(c.resPick.view(a.bodyHeight() - 4))
		}
	case stepSeverity:
		c.viewSeverity(&b)
	case stepContact:
		c.viewContact(a, &b)
	case stepAttach:
		b.WriteString(c.attach.View())
	case stepReview:
		c.viewReview(&b)
	}
	if c.err != nil {
		b.WriteString("\n" + errView(c.err))
	}
	if c.submitting {
		b.WriteString("\nSubmitting…")
	}
	b.WriteString("\n\ntab next field · ctrl+n next step · esc back · enter pick")
	return b.String()
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func (c *createScreen) viewSeverity(b *strings.Builder) {
	for i, l := range []string{"Minimal", "Moderate", "Critical", "Highest critical impact"} {
		mark := "  "
		if i == c.sev {
			mark = "> "
		}
		b.WriteString(mark + l + "\n")
	}
	b.WriteString("\nCritical and highest critical impact need a support plan that allows them; the API will reject them otherwise.\n")
	if c.critical() {
		b.WriteString("24x7 response (space): " + yesNo(c.require24) + "\n")
	}
	b.WriteString("Advanced diagnostic consent (c): " + yesNo(!c.noConsent))
}

func (c *createScreen) viewContact(a *app, b *strings.Builder) {
	if c.cpick != 0 {
		b.WriteString(c.cpicker.view(a.bodyHeight() - 4))
		return
	}
	if c.meNote != "" {
		b.WriteString(c.meNote + "\n")
	}
	f := c.contactFields()
	for i, id := range f {
		mark := "  "
		if i == c.cfield {
			mark = "> "
		}
		var line string
		switch id {
		case cfCountry:
			line = "Country: " + c.country + " (enter to change)"
		case cfZone:
			line = "Time zone: " + c.zone + " (enter to change)"
		case cfLang:
			line = "Language: " + c.lang + " (enter to change)"
		case cfMethod:
			line = "Contact method: " + string(c.method()) + " (space to toggle)"
		default:
			line = c.textInput(id).View()
		}
		b.WriteString(mark + line + "\n")
	}
}

func (c *createScreen) viewReview(b *strings.Builder) {
	d := c.ticketDetails(nil).Properties
	cp := d.ContactDetails
	fmt.Fprintf(b, "Title: %s\nService: %s\n", deref(d.Title), c.row.text())
	if c.resourceID != "" {
		fmt.Fprintf(b, "Resource: %s\n", c.resourceID)
	}
	fmt.Fprintf(b, "Severity: %s · 24x7: %s · Diagnostic consent: %s\n", *d.Severity, yesNo(d.Require24X7Response != nil), *d.AdvancedDiagnosticConsent)
	fmt.Fprintf(b, "Contact: %s %s <%s>, %s via %s\n", deref(cp.FirstName), deref(cp.LastName), deref(cp.PrimaryEmailAddress), deref(cp.Country), *cp.PreferredContactMethod)
	fmt.Fprintf(b, "Time zone: %s · Language: %s\n", deref(cp.PreferredTimeZone), deref(cp.PreferredSupportLanguage))
	if cp.PhoneNumber != nil {
		fmt.Fprintf(b, "Phone: %s\n", *cp.PhoneNumber)
	}
	if c.attachU != nil {
		fmt.Fprintf(b, "Attachment: %s\n", c.attachU.Name)
	}
	b.WriteString("\nenter to submit")
}
