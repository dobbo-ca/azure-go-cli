package support

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// deps is everything the TUI needs from the outside world; tests swap it.
type deps struct {
	newClient     func(Scope) (Client, error)
	loadProfile   func() (*config.Profile, error)
	discover      func(ctx context.Context) ([]azure.TenantInfo, error)
	signIn        func(ctx context.Context, tenantID string) ([]config.Subscription, error)
	listResources func(ctx context.Context, s Scope, types []string) ([]resourceRef, error)
	fetchMe       func(ctx context.Context, s Scope) (me, error)
	localZone     func() string
	getenv        func(string) string
}

func defaultDeps() deps {
	return deps{
		newClient:   NewClient,
		loadProfile: config.Load,
		discover:    azure.DiscoverAllSubscriptionsFromCache,
		signIn: func(ctx context.Context, tenantID string) ([]config.Subscription, error) {
			cred, err := azure.AuthenticateForTenant(ctx, tenantID)
			if err != nil {
				return nil, err
			}
			return azure.DiscoverTenantSubscriptions(ctx, tenantID, cred)
		},
		listResources: listResources,
		fetchMe: func(ctx context.Context, s Scope) (me, error) {
			cred, err := credentialFor(s.TenantID)
			if err != nil {
				return me{}, err
			}
			return fetchMe(ctx, cred)
		},
		localZone: localIANAZone,
		getenv:    os.Getenv,
	}
}

type screen interface {
	init(a *app) tea.Cmd // starts the screen's load; called by push, replace and setScope
	update(a *app, msg tea.Msg) tea.Cmd
	view(a *app) string
}

type app struct {
	ctx           context.Context
	d             deps
	scope         Scope
	client        Client
	err           error // last client-building or sign-in error
	gen           int   // incremented on every scope change; async msgs carry it
	stack         []screen
	catalog       []catalogRow // cached for the process
	catErr        error
	width, height int
	quitting      bool
}

// reauthDoneMsg is what the ctrl+l sign-in returns.
type reauthDoneMsg struct{ err error }

// newApp builds the app for a scope. A client that cannot be built (for
// example MFA needed) is kept in a.err so ctrl+l can sign in.
func newApp(ctx context.Context, d deps, s Scope) (*app, error) {
	a := &app{ctx: ctx, d: d, scope: s}
	a.client, a.err = d.newClient(s)
	a.stack = []screen{newListScreen()}
	return a, nil
}

func (a *app) top() screen { return a.stack[len(a.stack)-1] }

func (a *app) Init() tea.Cmd { return a.top().init(a) }

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
	case tea.KeyPressMsg:
		switch m.String() {
		case "ctrl+c":
			a.quitting = true
			return a, tea.Quit
		case "ctrl+l":
			return a, a.signInCmd(a.scope.TenantID, func(_ []config.Subscription, err error) tea.Msg { return reauthDoneMsg{err} })
		}
	case reauthDoneMsg:
		if m.err != nil {
			a.err = m.err
			return a, nil
		}
		// same scope: rebuild the client only, so an in-progress draft survives;
		// on failure keep the old client so the screens never see a nil one
		c, err := a.d.newClient(a.scope)
		if err != nil {
			a.err = err
			return a, nil
		}
		a.client, a.err = c, nil
		return a, a.top().init(a)
	}
	if gen, name, err, ok := detailMsg(msg); ok && gen == a.gen {
		// the detail screen may be covered or already left; never drop a failure
		for i := len(a.stack) - 1; i >= 0; i-- {
			if d, ok := a.stack[i].(*detailScreen); ok && d.name == name {
				return a, d.update(a, msg)
			}
		}
		if err != nil {
			a.err = err
		}
		return a, nil
	}
	if _, ok := msg.(ticketsLoadedMsg); ok {
		// the list is always the bottom screen; it may be covered while loading
		return a, a.stack[0].update(a, msg)
	}
	return a, a.top().update(a, msg)
}

// detailMsg unpacks the messages that belong to one ticket's detail screen.
func detailMsg(msg tea.Msg) (gen int, name string, err error, ok bool) {
	switch m := msg.(type) {
	case ticketLoadedMsg:
		return m.gen, m.name, m.err, true
	case ticketChangedMsg:
		return m.gen, m.name, m.err, true
	}
	return 0, "", nil, false
}

func (a *app) View() tea.View {
	if a.quitting {
		return tea.NewView("")
	}
	var b strings.Builder
	b.WriteString(a.scope.Label() + "\n")
	if a.err != nil {
		b.WriteString(errView(a.err) + "\n")
	}
	b.WriteString(a.top().view(a) + "\n\nctrl+l sign in · ctrl+c quit")
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// bodyHeight is the number of lines left for a screen's own content.
func (a *app) bodyHeight() int { return max(a.height-6, 5) }

func (a *app) push(s screen) tea.Cmd {
	a.stack = append(a.stack, s)
	return s.init(a)
}

func (a *app) pop() {
	if len(a.stack) > 1 {
		a.stack = a.stack[:len(a.stack)-1]
	}
}

func (a *app) replace(s screen) tea.Cmd {
	a.pop()
	return a.push(s)
}

// setScope switches scope without touching the profile. Results from the old
// scope are dropped because their gen no longer matches.
func (a *app) setScope(s Scope) tea.Cmd {
	a.scope = s
	a.client, a.err = a.d.newClient(s)
	a.gen++
	l := newListScreen()
	a.stack = []screen{l}
	return l.init(a)
}

// signInCmd releases the terminal for an interactive sign-in of one tenant.
func (a *app) signInCmd(tenantID string, done func([]config.Subscription, error) tea.Msg) tea.Cmd {
	c := &signInCommand{ctx: a.ctx, d: a.d, tenantID: tenantID}
	return tea.Exec(c, func(err error) tea.Msg { return done(c.subs, err) })
}

// signInCommand is a tea.ExecCommand; the setters are no-ops as sign-in opens a browser.
type signInCommand struct {
	ctx      context.Context
	d        deps
	tenantID string
	subs     []config.Subscription
}

func (c *signInCommand) Run() (err error) {
	c.subs, err = c.d.signIn(c.ctx, c.tenantID)
	return err
}
func (c *signInCommand) SetStdin(io.Reader)  {}
func (c *signInCommand) SetStdout(io.Writer) {}
func (c *signInCommand) SetStderr(io.Writer) {}

// errView renders an error verbatim, with a sign-in hint when that would fix it.
func errView(err error) string {
	s := "Error: " + err.Error()
	if azure.NeedsInteractiveAuth(err) {
		s += "\npress ctrl+l to sign in to this tenant"
	}
	return s
}

// startScope picks the scope the TUI opens in: --tenant, --subscription, else the default subscription.
func startScope(cmd *cobra.Command, p *config.Profile) (Scope, error) {
	if t, _ := cmd.Flags().GetString("tenant"); t != "" {
		return Scope{TenantID: t}, nil
	}
	want, _ := cmd.Flags().GetString("subscription")
	var pick *config.Subscription
	for i, sub := range p.Subscriptions {
		if want != "" && (sub.ID == want || strings.EqualFold(sub.Name, want)) ||
			want == "" && sub.IsDefault {
			pick = &p.Subscriptions[i]
			break
		}
	}
	if pick == nil && want == "" && len(p.Subscriptions) > 0 {
		pick = &p.Subscriptions[0]
	}
	if pick == nil {
		if want != "" {
			return Scope{}, fmt.Errorf("subscription %q not found in the profile", want)
		}
		return Scope{}, errors.New("no subscriptions in the profile; run 'az login'")
	}
	return Scope{TenantID: pick.TenantID, SubscriptionID: pick.ID, SubscriptionName: pick.Name}, nil
}

func runTUI(cmd *cobra.Command) error {
	p, err := config.Load()
	if err != nil {
		return err
	}
	s, err := startScope(cmd, p)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	a, err := newApp(ctx, defaultDeps(), s)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(a, tea.WithContext(ctx)).Run()
	return err
}
