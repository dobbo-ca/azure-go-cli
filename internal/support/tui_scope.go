package support

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

type rowKind int

const (
	rowHeading rowKind = iota
	rowScope
	rowOther  // "Other tenant…"
	rowSignIn // a tenant that needs interactive sign-in
)

type scopeRow struct {
	text  string
	kind  rowKind
	scope Scope
}

type scopeScreen struct {
	p       picker
	rows    []scopeRow
	loading bool
	spin    spinner.Model
	err     error
	names   map[string]string // tenant ID -> display name, from discovery
}

type tenantsLoadedMsg struct {
	gen     int
	tenants []azure.TenantInfo
	err     error
}

type tenantSignedInMsg struct {
	tenantID string
	subs     []config.Subscription
	err      error
}

const otherTenant = "Other tenant…"

func tenantLevelRow(id, name string) scopeRow {
	return scopeRow{"  Tenant-level tickets (no subscription) · " + id, rowScope, Scope{TenantID: id, TenantName: name}}
}

func subRow(tenantID, name string, s config.Subscription) scopeRow {
	return scopeRow{"  " + s.Name + " (" + s.ID + ")", rowScope, Scope{TenantID: tenantID, TenantName: name, SubscriptionID: s.ID, SubscriptionName: s.Name}}
}

// newScopeScreen lists the profile's tenants and subscriptions, tenants in first-seen order.
func newScopeScreen(p *config.Profile) *scopeScreen {
	var order []string
	subs := map[string][]config.Subscription{}
	for _, s := range p.Subscriptions {
		if _, ok := subs[s.TenantID]; !ok {
			order = append(order, s.TenantID)
		}
		subs[s.TenantID] = append(subs[s.TenantID], s)
	}
	var rows []scopeRow
	for _, t := range order {
		rows = append(rows, scopeRow{text: "Tenant " + t}, tenantLevelRow(t, ""))
		for _, s := range subs[t] {
			rows = append(rows, subRow(t, "", s))
		}
	}
	rows = append(rows, scopeRow{text: otherTenant, kind: rowOther})
	sc := &scopeScreen{names: map[string]string{}, spin: spinner.New()}
	sc.setRows(rows)
	return sc
}

func (sc *scopeScreen) setRows(rows []scopeRow) {
	sc.rows = rows
	items := make([]string, len(rows))
	for i, r := range rows {
		items[i] = r.text
	}
	sc.p = newPicker(items, "")
}

func (sc *scopeScreen) init(a *app) tea.Cmd { return nil }

func (sc *scopeScreen) update(a *app, msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tenantsLoadedMsg:
		if m.gen != a.gen {
			return nil
		}
		sc.loading = false
		sc.err = m.err
		if m.err != nil {
			return nil
		}
		var rows []scopeRow
		for _, t := range m.tenants {
			sc.names[t.TenantID] = t.DisplayName
			label := t.DisplayName + " (" + t.TenantID + ")"
			if t.NeedsMFA {
				rows = append(rows, scopeRow{text: label + " — sign in", kind: rowSignIn, scope: Scope{TenantID: t.TenantID, TenantName: t.DisplayName}})
				continue
			}
			rows = append(rows, scopeRow{text: label}, tenantLevelRow(t.TenantID, t.DisplayName))
			for _, s := range t.Subscriptions {
				rows = append(rows, subRow(t.TenantID, t.DisplayName, s))
			}
		}
		sc.setRows(rows)
		return nil
	case tenantSignedInMsg:
		sc.err = m.err
		if m.err != nil {
			return nil
		}
		name := sc.names[m.tenantID]
		rows := []scopeRow{{text: "Tenant " + m.tenantID}, tenantLevelRow(m.tenantID, name)}
		for _, s := range m.subs {
			rows = append(rows, subRow(m.tenantID, name, s))
		}
		sc.setRows(rows)
		return nil
	case spinner.TickMsg:
		if !sc.loading {
			return nil
		}
		var cmd tea.Cmd
		sc.spin, cmd = sc.spin.Update(m)
		return cmd
	case tea.KeyPressMsg:
		if m.String() == "esc" {
			a.pop()
			return nil
		}
	}
	i, done, cmd := sc.p.update(msg)
	if !done {
		return cmd
	}
	switch r := sc.rows[i]; r.kind {
	case rowScope:
		return a.setScope(r.scope)
	case rowOther:
		sc.loading, sc.err = true, nil
		gen := a.gen
		return tea.Batch(func() tea.Msg {
			t, err := a.d.discover(a.ctx)
			return tenantsLoadedMsg{gen, t, err}
		}, sc.spin.Tick)
	case rowSignIn:
		return a.signInCmd(r.scope.TenantID, func(subs []config.Subscription, err error) tea.Msg {
			return tenantSignedInMsg{r.scope.TenantID, subs, err}
		})
	}
	return nil
}

func (sc *scopeScreen) view(a *app) string {
	var b strings.Builder
	b.WriteString("Switch scope (enter selects, esc cancels)\n")
	switch {
	case sc.loading:
		b.WriteString(sc.spin.View() + " Loading tenants…")
	case sc.err != nil:
		b.WriteString(errView(sc.err) + "\n" + sc.p.view(a.bodyHeight()))
	default:
		b.WriteString(sc.p.view(a.bodyHeight()))
	}
	return b.String()
}
