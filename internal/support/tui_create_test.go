package support

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

func addService(fc *fakeClient, guid, display string, types []string, problems ...string) {
	var rt []*string
	for _, t := range types {
		rt = append(rt, ref(t))
	}
	fc.services = append(fc.services, &armsupport.Service{Name: &guid, Properties: &armsupport.ServiceProperties{DisplayName: &display, ResourceTypes: rt}})
	for i, p := range problems {
		id := "/providers/Microsoft.Support/services/" + guid + "/problemClassifications/00000000-0000-0000-0000-00000000000" + string(rune('1'+i))
		fc.problems[guid] = append(fc.problems[guid], &armsupport.ProblemClassification{ID: &id, Name: ref(id[len(id)-36:]), Properties: &armsupport.ProblemClassificationProperties{DisplayName: ref(p)}})
	}
}

const (
	vmGUID      = "aaaaaaaa-0000-0000-0000-000000000001"
	billingGUID = "517f2da6-78fd-0498-4e22-ad26996b1dfc"
	quotaGUID   = "06bfd9d3-516b-d5c6-5802-169c800dec89"
)

func createFake() *fakeClient {
	fc := newFake()
	addService(fc, vmGUID, "Virtual Machine", []string{"Microsoft.Compute/virtualMachines"}, "Cannot connect after reboot", "Performance")
	addService(fc, billingGUID, "Billing", nil, "Invoice question")
	addService(fc, quotaGUID, "Quota", nil, "Increase limit")
	return fc
}

func createApp(t *testing.T, fc *fakeClient) (*app, *createScreen) {
	t.Helper()
	a := newTestApp(t, fc)
	a.stack = []screen{newListScreen()}
	c := newCreateScreen()
	driveCmd(t, a, a.push(c))
	return a, c
}

// toService fills the describe step and moves to the service step.
func toService(t *testing.T, a *app, title string) {
	t.Helper()
	drive(t, a, keys(title, "tab", "testing", "ctrl+n")...)
}

func TestCreateDescribeRequiresTitle(t *testing.T) {
	a := listApp(t, newFake())
	drive(t, a, keys("n", "ctrl+n")...)
	c := a.top().(*createScreen)
	if !strings.Contains(view(a), "Title is required") || c.step != stepDescribe {
		t.Fatalf("view:\n%s", view(a))
	}
}

func TestCreateServiceSearchPrefill(t *testing.T) {
	a, c := createApp(t, createFake())
	toService(t, a, "Cannot connect to VM after reboot")
	if c.step != stepService || c.svc.input.Value() != "connect reboot" {
		t.Fatalf("step %v query %q", c.step, c.svc.input.Value())
	}
	if first := c.svc.items[c.svc.shown[0]]; first != "Virtual Machine › Cannot connect after reboot" {
		t.Fatalf("first = %q", first)
	}
}

func TestCreateServiceWhileLoading(t *testing.T) {
	a := newTestApp(t, createFake())
	c := newCreateScreen()
	a.push(c) // init Cmd withheld: the catalog never arrives
	toService(t, a, "Cannot connect to VM after reboot")
	drive(t, a, keys(" disk")...)
	if c.svc.input.Value() != "connect reboot disk" || !strings.Contains(view(a), "Loading") {
		t.Fatalf("query %q view:\n%s", c.svc.input.Value(), view(a))
	}
}

func TestCreateQuotaShowsHint(t *testing.T) {
	a, c := createApp(t, createFake())
	toService(t, a, "Quota please")
	drive(t, a, keys("limit", "enter")...)
	if !strings.Contains(view(a), quotaHint) || c.step != stepService {
		t.Fatalf("step %v view:\n%s", c.step, view(a))
	}
}

func TestCreateResourceStepSkippedForBilling(t *testing.T) {
	a, c := createApp(t, createFake())
	toService(t, a, "Invoice trouble")
	drive(t, a, keys("invoice", "enter")...)
	if c.step != stepSeverity || c.row == nil || c.row.ServiceName != billingGUID {
		t.Fatalf("step %v", c.step)
	}
}

func TestCreateResourceStepTenantLevelSkipped(t *testing.T) {
	fc := createFake()
	a, c := createApp(t, fc)
	a.scope = Scope{TenantID: "t2"}
	toService(t, a, "Cannot connect")
	drive(t, a, keys("enter")...)
	if c.step != stepSeverity {
		t.Fatalf("step %v", c.step)
	}
}

func TestCreateResourcePick(t *testing.T) {
	fc := createFake()
	a, c := createApp(t, fc)
	vmID := "/subscriptions/s2/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1"
	a.d.listResources = func(context.Context, Scope, []string) ([]resourceRef, error) {
		return []resourceRef{{ID: vmID, Name: "vm1", Type: "Microsoft.Compute/virtualMachines"}}, nil
	}
	toService(t, a, "Cannot connect to VM after reboot")
	drive(t, a, keys("enter")...)
	if c.step != stepResource {
		t.Fatalf("step %v", c.step)
	}
	drive(t, a, keys("vm1", "enter")...)
	if c.step != stepSeverity || c.resourceID != vmID {
		t.Fatalf("step %v resource %q", c.step, c.resourceID)
	}
}

func TestCreateEscKeepsValues(t *testing.T) {
	a, c := createApp(t, createFake())
	toService(t, a, "Cannot connect")
	drive(t, a, keys("esc")...)
	if c.step != stepDescribe || c.title.Value() != "Cannot connect" {
		t.Fatalf("step %v title %q", c.step, c.title.Value())
	}
}

func adaMe(a *app) {
	a.d.fetchMe = func(context.Context, Scope) (me, error) {
		return me{GivenName: "Ada", Surname: "Lovelace", Mail: "ada@x.com", UsageLocation: "GB"}, nil
	}
	a.d.localZone = func() string { return "Europe/London" }
	a.d.getenv = func(k string) string {
		if k == "LANG" {
			return "fr_FR.UTF-8"
		}
		return ""
	}
}

// toSeverity runs the describe and service steps for the billing row.
func toSeverity(t *testing.T, a *app) {
	t.Helper()
	toService(t, a, "Billing question - please ignore")
	drive(t, a, keys("enter")...)
}

func TestCreateEndToEnd(t *testing.T) {
	stubName(t, "11111111-1111-1111-1111-111111111111")
	fc := createFake()
	a, c := createApp(t, fc)
	adaMe(a)
	toSeverity(t, a)
	drive(t, a, keys("enter")...)  // severity Minimal -> contact
	drive(t, a, keys("ctrl+n")...) // contact -> attach
	drive(t, a, keys("ctrl+n")...) // attach -> review
	if c.step != stepReview {
		t.Fatalf("step %v err %v", c.step, c.err)
	}
	drive(t, a, keys("enter")...)
	tk, ok := fc.tickets["11111111-1111-1111-1111-111111111111"]
	if !ok {
		t.Fatalf("no ticket; calls %v", fc.calls)
	}
	p := tk.Properties
	cp := p.ContactDetails
	if deref(p.Title) != "Billing question - please ignore" || deref(p.Description) != "testing" ||
		*p.Severity != armsupport.SeverityLevelMinimal || *p.AdvancedDiagnosticConsent != armsupport.ConsentYes || p.Require24X7Response != nil ||
		deref(p.ServiceID) != "/providers/Microsoft.Support/services/"+billingGUID ||
		deref(p.FileWorkspaceName) != "11111111-1111-1111-1111-111111111111" ||
		deref(cp.FirstName) != "Ada" || deref(cp.LastName) != "Lovelace" || deref(cp.PrimaryEmailAddress) != "ada@x.com" ||
		deref(cp.Country) != "GBR" || deref(cp.PreferredTimeZone) != "GMT Standard Time" || deref(cp.PreferredSupportLanguage) != "fr-fr" ||
		*cp.PreferredContactMethod != armsupport.PreferredContactMethodEmail {
		t.Fatalf("stored %+v contact %+v", p, cp)
	}
	d, ok := a.top().(*detailScreen)
	if !ok || d.name != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("top = %T", a.top())
	}
}

func TestCreateCriticalNeedsPhone(t *testing.T) {
	a, c := createApp(t, createFake())
	adaMe(a)
	toSeverity(t, a)
	drive(t, a, keys("down", "down", "ctrl+n", "ctrl+n")...)
	if !strings.Contains(view(a), "Phone number is required") || c.step != stepContact {
		t.Fatalf("step %v view:\n%s", c.step, view(a))
	}
	if p := c.ticketDetails(a).Properties; p.Require24X7Response == nil || !*p.Require24X7Response {
		t.Fatal("Require24X7Response not set")
	}
}

func TestCreateGraphFailureStillPrefillsLocale(t *testing.T) {
	a, c := createApp(t, createFake())
	adaMe(a)
	a.d.fetchMe = func(context.Context, Scope) (me, error) { return me{}, errString("403 forbidden") }
	toSeverity(t, a)
	drive(t, a, keys("enter")...)
	if c.first.Value() != "" || c.zone != "GMT Standard Time" || c.lang != "fr-fr" || !strings.Contains(view(a), "Could not read your profile from Microsoft Graph: 403 forbidden") {
		t.Fatalf("first %q zone %q lang %q\n%s", c.first.Value(), c.zone, c.lang, view(a))
	}
}

func TestCreateTechnicalResourceInRequest(t *testing.T) {
	a, c := createApp(t, createFake())
	adaMe(a)
	vmID := "/subscriptions/s2/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1"
	a.d.listResources = func(context.Context, Scope, []string) ([]resourceRef, error) {
		return []resourceRef{{ID: vmID, Name: "vm1", Type: "Microsoft.Compute/virtualMachines"}}, nil
	}
	toService(t, a, "Cannot connect to VM after reboot")
	drive(t, a, keys("enter", "vm1", "enter")...)
	if got := c.ticketDetails(a).Properties.TechnicalTicketDetails; got == nil || deref(got.ResourceID) != vmID {
		t.Fatalf("technical = %+v", got)
	}
}

func TestCreateSubmitErrorStaysOnReview(t *testing.T) {
	fc := createFake()
	a, c := createApp(t, fc)
	adaMe(a)
	fc.err["CreateTicket"] = errString("plan does not allow")
	toSeverity(t, a)
	drive(t, a, keys("enter", "ctrl+n", "ctrl+n", "enter")...)
	if c.step != stepReview || !strings.Contains(view(a), "Error: plan does not allow") {
		t.Fatalf("step %v\n%s", c.step, view(a))
	}
	drive(t, a, keys("enter")...)
	if n := countCalls(fc, "CreateTicket"); n != 2 {
		t.Fatalf("CreateTicket calls = %d", n)
	}
}

func TestCreateAttachValidatedBeforeSubmit(t *testing.T) {
	a, c := createApp(t, createFake())
	adaMe(a)
	path := filepath.Join(t.TempDir(), "a.bat")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	toSeverity(t, a)
	drive(t, a, keys("enter", "ctrl+n", path, "ctrl+n")...)
	if !strings.Contains(view(a), "The file extension is not supported.") || c.step != stepAttach {
		t.Fatalf("step %v\n%s", c.step, view(a))
	}
}
