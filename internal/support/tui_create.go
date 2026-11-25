package support

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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

	err error
}

type catalogLoadedMsg struct {
	rows []catalogRow
	err  error
}

type resourcesLoadedMsg struct {
	gen  int
	rows []resourceRef
	err  error
}

func newCreateScreen() *createScreen {
	c := &createScreen{title: newInput(), desc: newTextarea(), start: newInput(), resInput: newInput()}
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
		if m.gen != a.gen {
			return nil
		}
		c.resLoading, c.err, c.resources = false, m.err, m.rows
		c.resPick.setItems(c.resourceItems())
		return nil
	case tea.KeyPressMsg:
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
	}
	return nil
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
	case stepService:
		if !c.svcInit {
			c.svcInit = true
			c.svc = newPicker(catalogItems(a.catalog), queryFromTitle(c.title.Value()))
		}
	case stepResource:
		c.resManual, c.resources, c.resLoading = false, nil, true
		c.resPick = newPicker(c.resourceItems(), "")
		s, types, gen := a.scope, c.row.ResourceTypes, a.gen
		list := a.d.listResources
		return func() tea.Msg {
			rows, err := list(a.ctx, s, types)
			return resourcesLoadedMsg{gen, rows, err}
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
	}
	if c.err != nil {
		b.WriteString("\n" + errView(c.err))
	}
	b.WriteString("\n\ntab next field · ctrl+n next step · esc back · enter pick")
	return b.String()
}
