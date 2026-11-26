package support

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
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
		// A workspace left over from a rejected earlier attempt must not block a retry.
		if err := c.CheckName(ctx, ws, typeFileWorkspaces); err != nil {
			ws = newName()
			if err := c.CheckName(ctx, ws, typeFileWorkspaces); err != nil {
				return td, err
			}
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

// enumFlag is a string flag that only accepts the listed values.
type enumFlag struct {
	val     string
	allowed []string
}

func (e *enumFlag) String() string { return e.val }
func (e *enumFlag) Type() string   { return "string" }
func (e *enumFlag) Set(v string) error {
	for _, a := range e.allowed {
		if a == v {
			e.val = v
			return nil
		}
	}
	return fmt.Errorf("invalid value %q, allowed values: %s", v, strings.Join(e.allowed, ", "))
}

func addEnum(cmd *cobra.Command, name, usage string, required bool, allowed ...string) {
	cmd.Flags().Var(&enumFlag{allowed: allowed}, name, usage+" Allowed values: "+strings.Join(allowed, ", ")+".")
	if required {
		_ = cmd.MarkFlagRequired(name)
	}
}

func addStr(cmd *cobra.Command, name, usage string, required bool) {
	cmd.Flags().String(name, "", usage)
	if required {
		_ = cmd.MarkFlagRequired(name)
	}
}

func ref[T any](v T) *T { return &v }

func str(cmd *cobra.Command, name string) string { v, _ := cmd.Flags().GetString(name); return v }

// strp returns a pointer to the flag value, or nil when the flag was not given.
func strp(cmd *cobra.Command, name string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return ref(str(cmd, name))
}

// addContactFlags adds the --contact-* flags; required for create, optional for update.
func addContactFlags(cmd *cobra.Command, required bool) {
	addStr(cmd, "contact-country", "Country of the contact, ISO 3166-1 alpha-3 code.", required)
	addStr(cmd, "contact-first-name", "First name of the contact.", required)
	addStr(cmd, "contact-last-name", "Last name of the contact.", required)
	addEnum(cmd, "contact-method", "Preferred contact method.", required, "email", "phone")
	addStr(cmd, "contact-language", "Preferred support language, for example en-us.", required)
	addStr(cmd, "contact-timezone", "Preferred time zone, a Windows time zone name.", required)
	addStr(cmd, "contact-email", "Primary email address of the contact.", required)
	addStr(cmd, "contact-phone-number", "Phone number of the contact.", false)
	cmd.Flags().StringSlice("contact-additional-emails", nil, "Additional email addresses to notify.")
}

// addListFlags adds Python's pagination flags (help text minus the backticks, which cobra reads as a value name). As in Python, --next-token is
// used only together with --max-items.
func addListFlags(cmd *cobra.Command) {
	cmd.Flags().Int("max-items", 0, "Total number of items to return in the command's output. If the total number of items available is more than the value specified, a token is provided in the command's output. To resume pagination, provide the token value in --next-token argument of a subsequent command.")
	cmd.Flags().String("next-token", "", "Token to specify where to start paginating. This is the token value from a previously truncated response.")
}

// paging reads --max-items and --next-token. A malformed token is an error even
// without --max-items, as Python validates it when parsing arguments.
func paging(cmd *cobra.Command) (int, *pageToken, error) {
	n, _ := cmd.Flags().GetInt("max-items")
	if n < 0 {
		return 0, nil, errors.New("--max-items must be at least 1")
	}
	s := str(cmd, "next-token")
	if s == "" {
		return n, nil, nil
	}
	tok, err := decodeToken(s)
	if err != nil {
		return 0, nil, fmt.Errorf("--next-token: %w", err)
	}
	return n, tok, nil
}

// printPage prints a list, then Python's warning with the token for the rest.
func printPage[T any](cmd *cobra.Command, items []*T, next *pageToken) error {
	if err := output.PrintJSON(cmd, listOrEmpty(items)); err != nil {
		return err
	}
	if next != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Token of next page: %s\n", next.encode())
	}
	return nil
}

func strPtrs(ss []string) []*string {
	out := make([]*string, len(ss))
	for i := range ss {
		out[i] = &ss[i]
	}
	return out
}

// readJSONArg returns s, or the file contents when s starts with "@".
func readJSONArg(s string) ([]byte, error) {
	if strings.HasPrefix(s, "@") {
		return os.ReadFile(s[1:])
	}
	return []byte(s), nil
}

// parseStartTime accepts RFC3339, "2006-01-02T15:04:05" (UTC) or "2006-01-02" (UTC).
func parseStartTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --start-time %q: use RFC3339, 2006-01-02T15:04:05 or 2006-01-02", s)
}

// parseSecondaryConsent parses a JSON array of {"type":..,"user-consent":"Yes|No"}, or "@file".
func parseSecondaryConsent(s string) ([]*armsupport.SecondaryConsent, error) {
	raw, err := readJSONArg(s)
	if err != nil {
		return nil, err
	}
	var in []struct {
		Type    string `json:"type"`
		Consent string `json:"user-consent"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("invalid --secondary-consent JSON: %w", err)
	}
	out := make([]*armsupport.SecondaryConsent, 0, len(in))
	for _, c := range in {
		if c.Consent != "Yes" && c.Consent != "No" {
			return nil, fmt.Errorf("invalid user-consent %q in --secondary-consent: use Yes or No", c.Consent)
		}
		out = append(out, &armsupport.SecondaryConsent{Type: ref(c.Type), UserConsent: ref(armsupport.UserConsent(c.Consent))})
	}
	return out, nil
}

// parseQuotaRequests parses a JSON array of {"payload":..,"region":..}, or "@file".
func parseQuotaRequests(s string) ([]*armsupport.QuotaChangeRequest, error) {
	raw, err := readJSONArg(s)
	if err != nil {
		return nil, err
	}
	var in []struct{ Payload, Region string }
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("invalid --quota-change-requests JSON: %w", err)
	}
	out := make([]*armsupport.QuotaChangeRequest, 0, len(in))
	for _, q := range in {
		out = append(out, &armsupport.QuotaChangeRequest{Payload: ref(q.Payload), Region: ref(q.Region)})
	}
	return out, nil
}

// ticketFromFlags builds the ticket to create from the create flags.
func ticketFromFlags(cmd *cobra.Command, tenantLevel bool) (armsupport.TicketDetails, error) {
	sev := armsupport.SeverityLevel(str(cmd, "severity"))
	method := armsupport.PreferredContactMethod(str(cmd, "contact-method"))
	phone := str(cmd, "contact-phone-number")
	if needsPhone(method, sev) && phone == "" {
		return armsupport.TicketDetails{}, errors.New("--contact-phone-number is required when --contact-method is phone or --severity is critical or highestcriticalimpact")
	}
	emails, _ := cmd.Flags().GetStringSlice("contact-additional-emails")
	p := &armsupport.TicketDetailsProperties{
		Title:                     ref(str(cmd, "title")),
		Description:               ref(str(cmd, "description")),
		ProblemClassificationID:   ref(str(cmd, "problem-classification")),
		Severity:                  &sev,
		AdvancedDiagnosticConsent: ref(armsupport.Consent(str(cmd, "advanced-diagnostic-consent"))),
		ContactDetails: &armsupport.ContactProfile{
			Country:                  ref(str(cmd, "contact-country")),
			FirstName:                ref(str(cmd, "contact-first-name")),
			LastName:                 ref(str(cmd, "contact-last-name")),
			PreferredContactMethod:   &method,
			PreferredSupportLanguage: ref(str(cmd, "contact-language")),
			PreferredTimeZone:        ref(str(cmd, "contact-timezone")),
			PrimaryEmailAddress:      ref(str(cmd, "contact-email")),
			AdditionalEmailAddresses: strPtrs(emails),
			PhoneNumber:              strp(cmd, "contact-phone-number"),
		},
		EnrollmentID: strp(cmd, "enrollment-id"),
	}
	if b, _ := cmd.Flags().GetBool("require-24-by-7-response"); b {
		p.Require24X7Response = &b
	}
	if cmd.Flags().Changed("start-time") {
		t, err := parseStartTime(str(cmd, "start-time"))
		if err != nil {
			return armsupport.TicketDetails{}, err
		}
		p.ProblemStartTime = &t
	}
	if cmd.Flags().Changed("secondary-consent") {
		sc, err := parseSecondaryConsent(str(cmd, "secondary-consent"))
		if err != nil {
			return armsupport.TicketDetails{}, err
		}
		p.SecondaryConsent = sc
	}
	if tenantLevel {
		p.FileWorkspaceName = strp(cmd, "file-workspace-name")
	} else {
		p.FileWorkspaceName = strp(cmd, "file-workspace")
		if cmd.Flags().Changed("quota-change-requests") || cmd.Flags().Changed("quota-change-subtype") || cmd.Flags().Changed("quota-change-version") {
			q := &armsupport.QuotaTicketDetails{QuotaChangeRequestSubType: strp(cmd, "quota-change-subtype"), QuotaChangeRequestVersion: strp(cmd, "quota-change-version")}
			if cmd.Flags().Changed("quota-change-requests") {
				reqs, err := parseQuotaRequests(str(cmd, "quota-change-requests"))
				if err != nil {
					return armsupport.TicketDetails{}, err
				}
				q.QuotaChangeRequests = reqs
			}
			p.QuotaTicketDetails = q
		}
	}
	return armsupport.TicketDetails{Properties: p}, nil
}

// updateFromFlags builds an update from the flags the user actually set.
func updateFromFlags(cmd *cobra.Command) armsupport.UpdateSupportTicket {
	var u armsupport.UpdateSupportTicket
	if cmd.Flags().Changed("advanced-diagnostic-consent") {
		u.AdvancedDiagnosticConsent = ref(armsupport.Consent(str(cmd, "advanced-diagnostic-consent")))
	}
	if cmd.Flags().Changed("severity") {
		u.Severity = ref(armsupport.SeverityLevel(str(cmd, "severity")))
	}
	if cmd.Flags().Changed("status") {
		u.Status = ref(armsupport.Status(str(cmd, "status")))
	}
	var cp armsupport.UpdateContactProfile
	any := false
	for name, set := range map[string]func(){
		"contact-country":      func() { cp.Country = strp(cmd, "contact-country") },
		"contact-first-name":   func() { cp.FirstName = strp(cmd, "contact-first-name") },
		"contact-last-name":    func() { cp.LastName = strp(cmd, "contact-last-name") },
		"contact-language":     func() { cp.PreferredSupportLanguage = strp(cmd, "contact-language") },
		"contact-timezone":     func() { cp.PreferredTimeZone = strp(cmd, "contact-timezone") },
		"contact-email":        func() { cp.PrimaryEmailAddress = strp(cmd, "contact-email") },
		"contact-phone-number": func() { cp.PhoneNumber = strp(cmd, "contact-phone-number") },
		"contact-method": func() {
			cp.PreferredContactMethod = ref(armsupport.PreferredContactMethod(str(cmd, "contact-method")))
		},
		"contact-additional-emails": func() {
			emails, _ := cmd.Flags().GetStringSlice("contact-additional-emails")
			cp.AdditionalEmailAddresses = strPtrs(emails)
		},
	} {
		if cmd.Flags().Changed(name) {
			set()
			any = true
		}
	}
	if any {
		u.ContactDetails = &cp
	}
	return u
}

func newTicketsCmd(tenantLevel bool) *cobra.Command {
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a support ticket. A file workspace is created for the ticket unless one is given.",
		RunE: func(cmd *cobra.Command, args []string) error {
			td, err := ticketFromFlags(cmd, tenantLevel)
			if err != nil {
				return err
			}
			c, sessionSub, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			if resource := str(cmd, "technical-resource"); resource != "" {
				// Like Python: always send it, validate only for technical services.
				if service, _, ok := parseProblemClassificationID(*td.Properties.ProblemClassificationID); ok && isTechnicalService(service) {
					if err := validateTechnicalResource(resource, sessionSub); err != nil {
						return err
					}
				}
				td.Properties.TechnicalTicketDetails = &armsupport.TechnicalTicketDetails{ResourceID: &resource}
			}
			t, err := createTicket(cmd.Context(), c, str(cmd, "ticket-name"), td, nil)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, t)
		},
	}
	addStr(create, "ticket-name", "Support ticket name.", true)
	addEnum(create, "advanced-diagnostic-consent", "Consent to collect advanced diagnostic information.", true, "No", "Yes")
	addStr(create, "description", "Detailed description of the issue.", true)
	addStr(create, "problem-classification", "Problem classification ID, from 'az support services problem-classifications list'.", true)
	addEnum(create, "severity", "Severity of the issue.", true, "critical", "highestcriticalimpact", "minimal", "moderate")
	addStr(create, "title", "Title of the support ticket.", true)
	addStr(create, "start-time", "Time the problem started: RFC3339, 2006-01-02T15:04:05 or 2006-01-02 (UTC).", false)
	create.Flags().Bool("require-24-by-7-response", false, "Require 24x7 response from Microsoft (only for critical severity).")
	addStr(create, "secondary-consent", `Secondary consents as a JSON array [{"type":"..","user-consent":"Yes|No"}], or @file. (The Python extension takes aaz shorthand.)`, false)
	addStr(create, "enrollment-id", "Enrollment ID of the Azure subscription.", false)
	addStr(create, "technical-resource", "Resource ID the technical ticket is about.", false)
	addContactFlags(create, true)
	if tenantLevel {
		addStr(create, "file-workspace-name", "Existing file workspace to attach. One is created when omitted.", false)
	} else {
		addStr(create, "file-workspace", "Existing file workspace to attach. One is created when omitted.", false)
		addStr(create, "quota-change-subtype", "Quota change request sub type.", false)
		addStr(create, "quota-change-version", "Quota change request version.", false)
		addStr(create, "quota-change-requests", `Quota change requests as a JSON array [{"payload":"..","region":".."}], or @file. (The Python extension takes aaz shorthand.)`, false)
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Update a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			u := updateFromFlags(cmd)
			if cmd.Flags().Changed("secondary-consent") {
				if u.SecondaryConsent, err = parseSecondaryConsent(str(cmd, "secondary-consent")); err != nil {
					return err
				}
			}
			t, err := c.UpdateTicket(cmd.Context(), str(cmd, "ticket-name"), u)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, t)
		},
	}
	addStr(update, "ticket-name", "Support ticket name.", true)
	addEnum(update, "advanced-diagnostic-consent", "Consent to collect advanced diagnostic information.", false, "No", "Yes")
	addEnum(update, "severity", "Severity of the issue.", false, "critical", "highestcriticalimpact", "minimal", "moderate")
	addEnum(update, "status", "Status of the ticket.", false, "closed", "open")
	addStr(update, "secondary-consent", `Secondary consents as a JSON array [{"type":"..","user-consent":"Yes|No"}], or @file.`, false)
	addContactFlags(update, false)

	list := &cobra.Command{
		Use:   "list",
		Short: "List support tickets. With no --filter and no --max-items, lists tickets created in the last 7 days.",
		RunE: func(cmd *cobra.Command, args []string) error {
			max, tok, err := paging(cmd)
			if err != nil {
				return err
			}
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			filter := str(cmd, "filter")
			if !cmd.Flags().Changed("filter") && !cmd.Flags().Changed("max-items") {
				filter = defaultListFilter(now())
			}
			items, next, err := c.ListTickets(cmd.Context(), filter, max, tok)
			if err != nil {
				return err
			}
			return printPage(cmd, items, next)
		},
	}
	addStr(list, "filter", "OData filter, for example \"Status eq 'Open'\" or \"CreatedDate ge 2026-11-01\".", false)
	addListFlags(list)

	show := &cobra.Command{
		Use:   "show",
		Short: "Show a support ticket",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			t, err := c.GetTicket(cmd.Context(), str(cmd, "ticket-name"))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, t)
		},
	}
	addStr(show, "ticket-name", "Support ticket name.", true)

	return group("tickets", "Manage support tickets", create, update, list, show)
}
