package support

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

func ticket(name, title string, day int) armsupport.TicketDetails {
	d := time.Date(2026, 11, day, 10, 0, 0, 0, time.UTC)
	return armsupport.TicketDetails{Name: &name, Properties: &armsupport.TicketDetailsProperties{
		Title: &title, CreatedDate: &d, Status: ref("Open"), SupportTicketID: ref("id-" + name),
	}}
}

func listApp(t *testing.T, fc *fakeClient) *app {
	t.Helper()
	a := newTestApp(t, fc)
	driveCmd(t, a, a.Init())
	return a
}

func TestListLoadsOpenTicketsNewestFirst(t *testing.T) {
	fc := newFake()
	fc.tickets["old"] = ticket("old", "Older ticket", 1)
	fc.tickets["new"] = ticket("new", "Newer ticket", 20)
	a := listApp(t, fc)
	v := view(a)
	if strings.Index(v, "Newer ticket") < 0 || strings.Index(v, "Newer ticket") > strings.Index(v, "Older ticket") {
		t.Fatalf("order wrong:\n%s", v)
	}
	if fc.lastFilter != "Status eq 'Open'" {
		t.Fatalf("filter = %q", fc.lastFilter)
	}
}

func TestListToggleFilter(t *testing.T) {
	old := now
	now = func() time.Time { return time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC) }
	defer func() { now = old }()
	fc := newFake()
	a := listApp(t, fc)
	drive(t, a, keys("f")...)
	if fc.lastFilter != "CreatedDate ge 2026-10-25" {
		t.Fatalf("filter = %q", fc.lastFilter)
	}
	if !strings.Contains(view(a), "all, last 30 days") {
		t.Fatalf("header not updated:\n%s", view(a))
	}
}

func TestListSearch(t *testing.T) {
	fc := newFake()
	fc.tickets["a"] = ticket("a", "VM will not boot", 2)
	fc.tickets["b"] = ticket("b", "Storage latency", 3)
	a := listApp(t, fc)
	drive(t, a, keys("/", "vm")...)
	v := view(a)
	if !strings.Contains(v, "VM will not boot") || strings.Contains(v, "Storage latency") {
		t.Fatalf("filter wrong:\n%s", v)
	}
}

func TestListShowsError(t *testing.T) {
	fc := newFake()
	fc.err["ListTickets"] = errors.New("boom")
	a := listApp(t, fc)
	if !strings.Contains(view(a), "Error: boom") {
		t.Fatalf("view:\n%s", view(a))
	}
}

func TestListIgnoresStaleScopeResults(t *testing.T) {
	a := listApp(t, newFake())
	a.gen = 2
	x := ticket("x", "Stale scope ticket", 5)
	drive(t, a, ticketsLoadedMsg{gen: 1, tickets: []*armsupport.TicketDetails{&x}})
	if strings.Contains(view(a), "Stale scope ticket") {
		t.Fatal("stale scope result shown")
	}
}

func TestListIgnoresStaleFilterResults(t *testing.T) {
	a := listApp(t, newFake())
	a.top().(*listScreen).all = true
	x := ticket("x", "Stale filter ticket", 5)
	drive(t, a, ticketsLoadedMsg{gen: a.gen, all: false, tickets: []*armsupport.TicketDetails{&x}})
	if strings.Contains(view(a), "Stale filter ticket") {
		t.Fatal("stale filter result shown")
	}
}
