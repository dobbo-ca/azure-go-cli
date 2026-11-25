package support

import (
	"context"
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
