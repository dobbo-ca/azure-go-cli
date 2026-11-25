package support

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/stretchr/testify/require"
)

const (
	svcGUID = "11111111-2222-3333-4444-555555555555"
	pcGUID  = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	pcID    = "/providers/Microsoft.Support/services/" + svcGUID + "/problemClassifications/" + pcGUID
)

func ptr[T any](v T) *T { return &v }

func fixNow(t *testing.T) time.Time {
	t.Helper()
	fixed := time.Date(2026, 11, 24, 10, 30, 15, 999, time.UTC)
	old, oldName := now, newName
	now = func() time.Time { return fixed }
	newName = func() string { return "00000000-0000-0000-0000-000000000000" }
	t.Cleanup(func() { now, newName = old, oldName })
	return fixed.Truncate(time.Second)
}

func newTicket() armsupport.TicketDetails {
	return armsupport.TicketDetails{Properties: &armsupport.TicketDetailsProperties{ProblemClassificationID: ptr(pcID)}}
}

func TestParseProblemClassificationID(t *testing.T) {
	svc, pc, ok := parseProblemClassificationID("/providers/Microsoft.Support/services/" + "11111111-2222-3333-4444-55555555AAAA" + "/problemClassifications/" + pcGUID)
	require.True(t, ok)
	require.Equal(t, "11111111-2222-3333-4444-55555555aaaa", svc)
	require.Equal(t, pcGUID, pc)
	_, _, ok = parseProblemClassificationID("/providers/Microsoft.Support/services/x/problemClassifications/y")
	require.False(t, ok)
}

func TestIsTechnicalService(t *testing.T) {
	require.False(t, isTechnicalService(billingServiceGUID))
	require.False(t, isTechnicalService(quotaServiceGUID))
	require.False(t, isTechnicalService(subMgmtServiceGUID))
	require.True(t, isTechnicalService(svcGUID))
	require.True(t, isQuotaService(quotaServiceGUID))
	require.False(t, isQuotaService(svcGUID))
}

func TestDefaultListFilter(t *testing.T) {
	require.Equal(t, "CreatedDate ge 2026-11-17", defaultListFilter(time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)))
}

func TestValidateTechnicalResource(t *testing.T) {
	sub := "11111111-1111-1111-1111-111111111111"
	other := "22222222-2222-2222-2222-222222222222"
	vm := func(s string) string {
		return "/subscriptions/" + s + "/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1"
	}
	require.NoError(t, validateTechnicalResource("", sub))
	require.EqualError(t, validateTechnicalResource("bad", sub), "Technical resource argument bad is invalid.")
	require.EqualError(t, validateTechnicalResource(vm(other), sub), "Technical resource argument "+vm(other)+" is invalid. "+other+" does not match with "+sub)
	require.EqualError(t, validateTechnicalResource(vm("notaguid"), sub), "Technical resource argument "+vm("notaguid")+" is invalid.Subscription id notaguid is invalid.")
	require.NoError(t, validateTechnicalResource(vm(sub), sub))
}

func TestNeedsPhone(t *testing.T) {
	require.False(t, needsPhone(armsupport.PreferredContactMethodEmail, armsupport.SeverityLevelMinimal))
	require.True(t, needsPhone(armsupport.PreferredContactMethodPhone, armsupport.SeverityLevelMinimal))
	require.True(t, needsPhone(armsupport.PreferredContactMethodEmail, armsupport.SeverityLevelCritical))
	require.True(t, needsPhone(armsupport.PreferredContactMethodEmail, armsupport.SeverityLevelHighestcriticalimpact))
}

func TestCreateTicketOrder(t *testing.T) {
	fixed := fixNow(t)
	f := newFake()
	_, err := createTicket(context.Background(), f, "abc-1", newTicket(), &upload{Name: "f.txt", Data: []byte("hi")})
	require.NoError(t, err)
	require.Equal(t, []string{
		"CheckName:abc-1", "CheckName:abc-1", "CreateFileWorkspace:abc-1",
		"CreateFile:abc-1/f.txt", "UploadChunk:abc-1/f.txt/0", "CreateTicket:abc-1",
	}, f.calls)
	p := f.tickets["abc-1"].Properties
	require.Equal(t, "/providers/Microsoft.Support/services/"+svcGUID, *p.ServiceID)
	require.Equal(t, "abc-1", *p.FileWorkspaceName)
	require.Equal(t, fixed, *p.ProblemStartTime)
}

func TestCreateTicketKeepsGivenWorkspace(t *testing.T) {
	fixNow(t)
	f := newFake()
	td := newTicket()
	td.Properties.FileWorkspaceName = ptr("ws1")
	_, err := createTicket(context.Background(), f, "abc-1", td, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"CheckName:abc-1", "CreateTicket:abc-1"}, f.calls)
	require.Equal(t, "ws1", *f.tickets["abc-1"].Properties.FileWorkspaceName)
}

func TestCreateTicketWorkspaceNameFallsBackToGUID(t *testing.T) {
	fixNow(t)
	f := newFake()
	_, err := createTicket(context.Background(), f, "my/ticket", newTicket(), nil)
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000000", *f.tickets["my/ticket"].Properties.FileWorkspaceName)
	require.True(t, f.workspaces["00000000-0000-0000-0000-000000000000"])
}

func TestCreateTicketNameTakenStopsEarly(t *testing.T) {
	fixNow(t)
	f := newFake()
	f.takenNames["abc-1"] = true
	_, err := createTicket(context.Background(), f, "abc-1", newTicket(), nil)
	require.EqualError(t, err, "Name taken")
	require.Equal(t, []string{"CheckName:abc-1"}, f.calls)
}

func TestCreateTicketInvalidPC(t *testing.T) {
	f := newFake()
	td := armsupport.TicketDetails{Properties: &armsupport.TicketDetailsProperties{ProblemClassificationID: ptr("nope")}}
	_, err := createTicket(context.Background(), f, "abc-1", td, nil)
	require.EqualError(t, err, errInvalidProblemClassification)
	require.Empty(t, f.calls)
}
