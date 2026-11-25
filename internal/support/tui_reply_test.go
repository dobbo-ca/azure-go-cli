package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

func stubName(t *testing.T, n string) {
	t.Helper()
	old := newName
	newName = func() string { return n }
	t.Cleanup(func() { newName = old })
}

func replyApp(t *testing.T, fc *fakeClient, mod func(*armsupport.TicketDetailsProperties)) *app {
	t.Helper()
	stubName(t, "c-1")
	a := detailApp(t, fc, mod)
	drive(t, a, keys("r")...)
	if _, ok := a.top().(*replyScreen); !ok {
		t.Fatalf("top = %T", a.top())
	}
	return a
}

func TestReplySends(t *testing.T) {
	fc := newFake()
	a := replyApp(t, fc, nil)
	fc.calls = nil
	drive(t, a, keys("tab", "Thanks", "ctrl+s")...)
	if got := strings.Join(fc.calls[:2], ","); got != "CheckCommunicationName:T1/c-1,CreateCommunication:T1/c-1" {
		t.Fatalf("calls %v", fc.calls)
	}
	c := fc.comms["T1"][0].Properties
	if deref(c.Subject) != "Detail title" || deref(c.Body) != "Thanks" {
		t.Fatalf("stored %+v", c)
	}
	if _, ok := a.top().(*detailScreen); !ok {
		t.Fatalf("top = %T", a.top())
	}
}

func TestReplyEmptyBody(t *testing.T) {
	fc := newFake()
	a := replyApp(t, fc, nil)
	fc.calls = nil
	drive(t, a, keys("ctrl+s")...)
	if !strings.Contains(view(a), "Message body is required") || len(fc.calls) != 0 {
		t.Fatalf("calls %v\n%s", fc.calls, view(a))
	}
}

func TestReplyErrorKeepsText(t *testing.T) {
	fc := newFake()
	a := replyApp(t, fc, nil)
	fc.err["CreateCommunication"] = errString("denied")
	drive(t, a, keys("tab", "Thanks", "ctrl+s")...)
	if v := view(a); !strings.Contains(v, "Error: denied") || !strings.Contains(v, "Thanks") {
		t.Fatalf("view:\n%s", v)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestReplyAttachBadFile(t *testing.T) {
	fc := newFake()
	a := replyApp(t, fc, func(p *armsupport.TicketDetailsProperties) { p.FileWorkspaceName = ref("ws1") })
	fc.calls = nil
	path := filepath.Join(t.TempDir(), "x.exe")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	drive(t, a, keys("tab", "hi", "tab", path, "ctrl+s")...)
	if !strings.Contains(view(a), "The file extension is not supported.") || len(fc.calls) != 0 {
		t.Fatalf("calls %v\n%s", fc.calls, view(a))
	}
}

func TestReplyAttachNoWorkspace(t *testing.T) {
	fc := newFake()
	a := replyApp(t, fc, nil)
	fc.calls = nil
	drive(t, a, keys("tab", "x", "tab", "/tmp/whatever.txt", "ctrl+s")...)
	if !strings.Contains(view(a), "this ticket has no file workspace") ||
		countCalls(fc, "CheckCommunicationName") != 0 || countCalls(fc, "CreateFile") != 0 {
		t.Fatalf("calls %v\n%s", fc.calls, view(a))
	}
}
