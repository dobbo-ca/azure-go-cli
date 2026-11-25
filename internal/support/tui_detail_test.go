package support

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

func comm(name, body string, dir armsupport.CommunicationDirection, hour int) *armsupport.CommunicationDetails {
	d := time.Date(2026, 11, 20, hour, 0, 0, 0, time.UTC)
	return &armsupport.CommunicationDetails{Name: &name, Properties: &armsupport.CommunicationDetailsProperties{
		Body: &body, Subject: ref("Re: " + name), Sender: ref("sender-" + name), CommunicationDirection: &dir, CreatedDate: &d,
	}}
}

// detailApp opens the detail screen for ticket "T1" with the given properties.
func detailApp(t *testing.T, fc *fakeClient, mod func(*armsupport.TicketDetailsProperties)) *app {
	t.Helper()
	tk := ticket("T1", "Detail title", 20)
	if mod != nil {
		mod(tk.Properties)
	}
	fc.tickets["T1"] = tk
	a := newTestApp(t, fc)
	driveCmd(t, a, a.push(newDetailScreen("T1")))
	return a
}

func countCalls(fc *fakeClient, prefix string) (n int) {
	for _, c := range fc.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return
}

func TestRenderThreadOrderAndHTML(t *testing.T) {
	tk := ticket("T1", "Title", 20)
	out := renderThread(tk, []*armsupport.CommunicationDetails{
		comm("later", "my reply", armsupport.CommunicationDirectionOutbound, 12),
		comm("first", "<p>Hi&amp;bye</p>", armsupport.CommunicationDirectionInbound, 9),
	})
	a, b := strings.Index(out, "from Microsoft"), strings.Index(out, "from you")
	if a < 0 || b < 0 || a > b {
		t.Fatalf("order wrong:\n%s", out)
	}
	if !strings.Contains(out, "Hi&bye") {
		t.Fatalf("html not converted:\n%s", out)
	}
}

func TestDetailCloseTicket(t *testing.T) {
	fc := newFake()
	a := detailApp(t, fc, nil)
	drive(t, a, keys("s", "y")...)
	u := fc.updates[len(fc.updates)-1]
	if u.Status == nil || *u.Status != armsupport.StatusClosed {
		t.Fatalf("update = %+v", u)
	}
	if n := countCalls(fc, "GetTicket"); n < 2 {
		t.Fatalf("no reload, GetTicket calls = %d", n)
	}
}

func TestDetailReopenTicket(t *testing.T) {
	fc := newFake()
	a := detailApp(t, fc, func(p *armsupport.TicketDetailsProperties) { p.Status = ref("Closed") })
	drive(t, a, keys("s", "y")...)
	if u := fc.updates[len(fc.updates)-1]; u.Status == nil || *u.Status != armsupport.StatusOpen {
		t.Fatalf("update = %+v", u)
	}
}

func TestDetailSeverityNeedsPhone(t *testing.T) {
	fc := newFake()
	a := detailApp(t, fc, nil)
	drive(t, a, keys("v", "critical", "enter", "+15550100", "enter")...)
	u := fc.updates[len(fc.updates)-1]
	if u.Severity == nil || *u.Severity != armsupport.SeverityLevelCritical ||
		u.ContactDetails == nil || deref(u.ContactDetails.PhoneNumber) != "+15550100" {
		t.Fatalf("update = %+v", u)
	}
}

func TestDetailAttachNoWorkspace(t *testing.T) {
	fc := newFake()
	a := detailApp(t, fc, nil)
	drive(t, a, keys("a")...)
	if !strings.Contains(view(a), "this ticket has no file workspace") || countCalls(fc, "CreateFile") != 0 {
		t.Fatalf("view:\n%s", view(a))
	}
}

func TestDetailAttachUploads(t *testing.T) {
	fc := newFake()
	a := detailApp(t, fc, func(p *armsupport.TicketDetailsProperties) { p.FileWorkspaceName = ref("ws1") })
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	drive(t, a, keys("a", path, "enter")...)
	if countCalls(fc, "CreateFile:ws1/note.txt") != 1 || !strings.Contains(view(a), "Uploaded note.txt") {
		t.Fatalf("calls %v\n%s", fc.calls, view(a))
	}
}

func TestDetailIgnoresOtherTicketLoad(t *testing.T) {
	a := detailApp(t, newFake(), nil)
	x := ticket("X", "Other ticket title", 5)
	drive(t, a, ticketLoadedMsg{gen: a.gen, name: "X", ticket: &x})
	if !strings.Contains(view(a), "Detail title") || strings.Contains(view(a), "Other ticket title") {
		t.Fatal("other ticket's load shown")
	}
}

func TestDetailKeysIgnoredBeforeLoad(t *testing.T) {
	fc := newFake()
	fc.tickets["T1"] = ticket("T1", "Detail title", 20)
	a := newTestApp(t, fc)
	a.push(newDetailScreen("T1")) // load withheld
	drive(t, a, keys("s", "y", "v", "a")...)
	d := a.top().(*detailScreen)
	if len(fc.updates) != 0 || d.mode != modeView || d.err != nil {
		t.Fatalf("updates %v mode %v err %v", fc.updates, d.mode, d.err)
	}
}

func TestDetailChangeFailureShownAfterEsc(t *testing.T) {
	fc := newFake()
	fc.err["UpdateTicket"] = errors.New("forbidden-xyz")
	a := detailApp(t, fc, nil)
	drive(t, a, keys("s")...)
	_, cmd := a.Update(keys("y")[0])
	drive(t, a, keys("esc")...)
	driveCmd(t, a, cmd)
	if !strings.Contains(view(a), "forbidden-xyz") {
		t.Fatalf("error lost:\n%s", view(a))
	}
}

func TestDetailChangeUsesClientOfStartScope(t *testing.T) {
	fc, other := newFake(), newFake()
	a := detailApp(t, fc, nil)
	drive(t, a, keys("s")...)
	_, cmd := a.Update(keys("y")[0])
	a.client = other // scope switched while the update is in flight
	cmd()
	if len(fc.updates) != 1 || len(other.updates) != 0 {
		t.Fatalf("updates %d / %d", len(fc.updates), len(other.updates))
	}
}

func TestDetailPasteIntoAttachPath(t *testing.T) {
	a := detailApp(t, newFake(), func(p *armsupport.TicketDetailsProperties) { p.FileWorkspaceName = ref("ws1") })
	drive(t, a, keys("a")...)
	drive(t, a, tea.PasteMsg{Content: "/tmp/pasted.txt"})
	if !strings.Contains(view(a), "/tmp/pasted.txt") {
		t.Fatalf("paste ignored:\n%s", view(a))
	}
}

func TestDetailWrapsLongBody(t *testing.T) {
	fc := newFake()
	a := detailApp(t, fc, nil)
	a.width = 40
	d := a.top().(*detailScreen)
	d.vp.SetWidth(40)
	d.vp.SetContent(renderThread(fc.tickets["T1"], []*armsupport.CommunicationDetails{
		comm("c", "Hello, we have investigated your issue and found that the root cause is X. Please apply Y.", armsupport.CommunicationDirectionOutbound, 9),
	}))
	if v := view(a); !strings.Contains(v, "apply Y.") {
		t.Fatalf("body clipped:\n%s", v)
	}
}
