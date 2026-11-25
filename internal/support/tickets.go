package support

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/google/uuid"
)

const (
	billingServiceGUID              = "517f2da6-78fd-0498-4e22-ad26996b1dfc"
	quotaServiceGUID                = "06bfd9d3-516b-d5c6-5802-169c800dec89"
	subMgmtServiceGUID              = "f3dc5421-79ef-1efa-41a5-42bf3cbb52c6"
	errInvalidProblemClassification = "Problem classification id is invalid. Please use 'az support services problem-classifications list' to get the most recent set of problem classification ids for your service."
)

var (
	now     = time.Now       // test seam
	newName = uuid.NewString // test seam
)

const guidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

var problemClassificationRe = regexp.MustCompile(`^/providers/microsoft\.support/services/(` + guidPattern + `)/problemclassifications/(` + guidPattern + `)$`)

// parseProblemClassificationID splits a problem classification ID into its
// lower-cased service and problem GUIDs.
func parseProblemClassificationID(id string) (service, problem string, ok bool) {
	m := problemClassificationRe.FindStringSubmatch(strings.ToLower(id))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func serviceIDFor(service string) string { return "/providers/Microsoft.Support/services/" + service }

// isTechnicalService is true unless the service is billing, quota or subscription management.
func isTechnicalService(service string) bool {
	s := strings.ToLower(service)
	return !strings.Contains(s, billingServiceGUID) && !strings.Contains(s, quotaServiceGUID) && !strings.Contains(s, subMgmtServiceGUID)
}

func isQuotaService(service string) bool {
	return strings.Contains(strings.ToLower(service), quotaServiceGUID)
}

// defaultListFilter is the list filter used when none is given: the last 7 days.
func defaultListFilter(t time.Time) string {
	return "CreatedDate ge " + t.AddDate(0, 0, -7).Format("2006-01-02")
}

// validateTechnicalResource checks a technical ticket's resource ID against the
// session subscription, with the Python extension's messages.
func validateTechnicalResource(resourceID, sessionSub string) error {
	if resourceID == "" {
		return nil
	}
	base := "Technical resource argument " + resourceID + " is invalid."
	r, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return errors.New(base)
	}
	if _, err := uuid.Parse(r.SubscriptionID); err != nil {
		return errors.New(base + "Subscription id " + r.SubscriptionID + " is invalid.")
	}
	if r.SubscriptionID != sessionSub {
		return errors.New(base + " " + r.SubscriptionID + " does not match with " + sessionSub)
	}
	return nil
}

// needsPhone: a phone number is required for the phone contact method and for critical severities.
func needsPhone(method armsupport.PreferredContactMethod, sev armsupport.SeverityLevel) bool {
	return method == armsupport.PreferredContactMethodPhone ||
		sev == armsupport.SeverityLevelCritical || sev == armsupport.SeverityLevelHighestcriticalimpact
}

// createTicket validates the problem classification, makes sure the name is
// free, ensures a file workspace (creating one unless given), uploads the
// attachment and creates the ticket. It stops at the first error and does not
// roll back: an empty workspace expires on its own.
func createTicket(ctx context.Context, c Client, name string, td armsupport.TicketDetails, attach *upload) (armsupport.TicketDetails, error) {
	p := td.Properties
	if p == nil || p.ProblemClassificationID == nil {
		return td, errors.New(errInvalidProblemClassification)
	}
	service, _, ok := parseProblemClassificationID(*p.ProblemClassificationID)
	if !ok {
		return td, errors.New(errInvalidProblemClassification)
	}
	id := serviceIDFor(service)
	p.ServiceID = &id
	if p.ProblemStartTime == nil {
		t := now().UTC().Truncate(time.Second)
		p.ProblemStartTime = &t
	}
	if err := c.CheckName(ctx, name, armsupport.TypeMicrosoftSupportSupportTickets); err != nil {
		return td, err
	}
	if p.FileWorkspaceName == nil {
		ws := name
		if !workspaceNamePattern.MatchString(ws) {
			ws = newName()
		}
		if err := c.CheckName(ctx, ws, typeFileWorkspaces); err != nil {
			return td, err
		}
		if _, err := c.CreateFileWorkspace(ctx, ws); err != nil {
			return td, err
		}
		p.FileWorkspaceName = &ws
	}
	if attach != nil {
		if err := uploadFile(ctx, c, *p.FileWorkspaceName, *attach); err != nil {
			return td, err
		}
	}
	return c.CreateTicket(ctx, name, td)
}
