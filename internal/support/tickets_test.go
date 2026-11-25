package support

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/spf13/cobra"
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

// runCmd runs `az support <args>` against fake and returns stdout.
func runCmd(t *testing.T, fake *fakeClient, args ...string) (string, error) {
	t.Helper()
	old := commandClientFn
	commandClientFn = func(_ *cobra.Command, tenantLevel bool) (Client, string, error) {
		if tenantLevel {
			return fake, "", nil
		}
		return fake, "sub1", nil
	}
	t.Cleanup(func() { commandClientFn = old })
	cmd := NewSupportCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func createArgs(extra ...string) []string {
	return append([]string{"in-subscription", "tickets", "create",
		"--ticket-name", "t1", "--advanced-diagnostic-consent", "Yes", "--description", "d",
		"--problem-classification", pcID, "--severity", "moderate", "--title", "T",
		"--contact-country", "CAN", "--contact-first-name", "A", "--contact-last-name", "B",
		"--contact-method", "email", "--contact-language", "en-us",
		"--contact-timezone", "Pacific Standard Time", "--contact-email", "a@x"}, extra...)
}

func TestTicketsCreateBuildsDetails(t *testing.T) {
	fixNow(t)
	f := newFake()
	_, err := runCmd(t, f, createArgs("--contact-additional-emails", "a@x,b@y", "--require-24-by-7-response", "--start-time", "2026-11-20")...)
	require.NoError(t, err)
	p := f.tickets["t1"].Properties
	require.Equal(t, "T", *p.Title)
	require.Equal(t, "d", *p.Description)
	require.Equal(t, armsupport.SeverityLevelModerate, *p.Severity)
	require.Equal(t, armsupport.ConsentYes, *p.AdvancedDiagnosticConsent)
	cd := p.ContactDetails
	require.Equal(t, "CAN", *cd.Country)
	require.Equal(t, armsupport.PreferredContactMethodEmail, *cd.PreferredContactMethod)
	require.Equal(t, "Pacific Standard Time", *cd.PreferredTimeZone)
	require.Equal(t, "en-us", *cd.PreferredSupportLanguage)
	require.Equal(t, []*string{ptr("a@x"), ptr("b@y")}, cd.AdditionalEmailAddresses)
	require.True(t, *p.Require24X7Response)
	require.Equal(t, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), *p.ProblemStartTime)
}

func TestTicketsCreateTechnicalResourceMismatch(t *testing.T) {
	fixNow(t)
	f := newFake()
	res := "/subscriptions/99999999-9999-9999-9999-999999999999/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm"
	_, err := runCmd(t, f, createArgs("--technical-resource", res)...)
	require.ErrorContains(t, err, "does not match with sub1")
	require.Empty(t, f.calls)
}

func TestTicketsCreateQuotaFlags(t *testing.T) {
	fixNow(t)
	f := newFake()
	_, err := runCmd(t, f, createArgs("--quota-change-requests", `[{"payload":"{}","region":"eastus"}]`)...)
	require.NoError(t, err)
	require.Equal(t, "eastus", *f.tickets["t1"].Properties.QuotaTicketDetails.QuotaChangeRequests[0].Region)
}

func TestTicketsCreateRejectsBadEnum(t *testing.T) {
	_, err := runCmd(t, newFake(), createArgs("--severity", "urgent")...)
	require.ErrorContains(t, err, "allowed values")
}

func TestTicketsCreateNeedsPhone(t *testing.T) {
	_, err := runCmd(t, newFake(), createArgs("--severity", "critical")...)
	require.ErrorContains(t, err, "--contact-phone-number is required")
}

func TestTicketsListDefaultFilter(t *testing.T) {
	fixNow(t)
	f := newFake()
	_, err := runCmd(t, f, "in-subscription", "tickets", "list")
	require.NoError(t, err)
	require.Equal(t, "CreatedDate ge 2026-11-17", f.lastFilter)
	_, err = runCmd(t, f, "in-subscription", "tickets", "list", "--filter", "Status eq 'Open'")
	require.NoError(t, err)
	require.Equal(t, "Status eq 'Open'", f.lastFilter)
	_, err = runCmd(t, f, "in-subscription", "tickets", "list", "--max-items", "5")
	require.NoError(t, err)
	require.Equal(t, "", f.lastFilter)
	for _, n := range []string{"a", "b", "c"} {
		f.tickets[n] = newTicket()
	}
	out, err := runCmd(t, f, "no-subscription", "tickets", "list", "--max-items", "2")
	require.NoError(t, err)
	var got []any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got, 2)
}

func TestTicketsUpdateOnlyChanged(t *testing.T) {
	cmd, _, err := NewSupportCommand().Find([]string{"in-subscription", "tickets", "update"})
	require.NoError(t, err)
	require.NoError(t, cmd.ParseFlags([]string{"--ticket-name", "t1", "--status", "closed"}))
	u := updateFromFlags(cmd)
	require.Equal(t, armsupport.StatusClosed, *u.Status)
	require.Nil(t, u.ContactDetails)
	require.Nil(t, u.Severity)
	require.NoError(t, cmd.ParseFlags([]string{"--contact-first-name", "Z"}))
	require.Equal(t, "Z", *updateFromFlags(cmd).ContactDetails.FirstName)
}

func TestParseStartTime(t *testing.T) {
	for _, s := range []string{"2026-11-20T01:02:03Z", "2026-11-20T01:02:03", "2026-11-20"} {
		_, err := parseStartTime(s)
		require.NoError(t, err, s)
	}
	_, err := parseStartTime("yesterday")
	require.Error(t, err)
}

func TestParseSecondaryConsent(t *testing.T) {
	got, err := parseSecondaryConsent(`[{"type":"x","user-consent":"Yes"}]`)
	require.NoError(t, err)
	require.Len(t, got, 1)
	_, err = parseSecondaryConsent(`[{"user-consent":"maybe"}]`)
	require.Error(t, err)
}
