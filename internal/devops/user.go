package devops

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// newUserCommand wires the `az devops user` command group: list, show,
// remove, update, add. Mirrors azext_devops/dev/team/user.py and its
// registration in dev/team/commands.py:133-138.
func newUserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage Azure DevOps users.",
	}

	cmd.AddCommand(userNewListCmd())
	cmd.AddCommand(userNewShowCmd())
	cmd.AddCommand(userNewRemoveCmd())
	cmd.AddCommand(userNewUpdateCmd())
	cmd.AddCommand(userNewAddCmd())

	return cmd
}

// userColumns is transform_user(s)_table_output / _transform_user_row
// (dev/team/_format.py:331-352): ID, Display Name, Email, License Type,
// Access Level, Status. Same row shape for list/show/update/add.
var userColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Display Name", Field: "user.displayName"},
	{Header: "Email", Field: "user.mailAddress"},
	{Header: "License Type", Field: "accessLevel.accountLicenseType"},
	{Header: "Access Level", Field: "accessLevel.licenseDisplayName"},
	{Header: "Status", Field: "accessLevel.status"},
}

// userLicenseTypes mirrors _LICENSE_TYPES (dev/team/arguments.py:22).
var userLicenseTypes = []string{"advanced", "earlyAdopter", "express", "professional", "stakeholder"}

func userLicenseTypeHelp() string {
	return fmt.Sprintf("Type of Account License. Allowed values: %s.", strings.Join(userLicenseTypes, ", "))
}

// userNormalizeLicenseType matches v against userLicenseTypes
// case-insensitively (arguments.py:110 registers license_type via
// get_enum_type(), whose CaseInsensitiveList choices match
// case-insensitively) and returns the canonical-cased value.
func userNormalizeLicenseType(v string) (string, error) {
	for _, t := range userLicenseTypes {
		if strings.EqualFold(v, t) {
			return t, nil
		}
	}
	return "", fmt.Errorf("--license-type must be one of %s", strings.Join(userLicenseTypes, ", "))
}

// userAddUserFlag registers the required --user flag shared by show/remove/update.
// Python declares no alias for it (dev/team/arguments.py has no
// `devops user show/remove/update` argument_context entry customizing it).
func userAddUserFlag(cmd *cobra.Command) {
	cmd.Flags().String("user", "", "Email ID or ID of the user.")
	cmd.MarkFlagRequired("user")
}

// userParseTriState parses a get_three_state_flag()-style flag value: caller
// only invokes this once the flag has actually been given a value ("" is
// handled by the caller as unset); "true"/"false" (case-insensitive) are the
// only accepted explicit values.
func userParseTriState(flag, v string) (bool, error) {
	switch {
	case strings.EqualFold(v, "true"):
		return true, nil
	case strings.EqualFold(v, "false"):
		return false, nil
	default:
		return false, fmt.Errorf("invalid value %q for --%s; must be true or false", v, flag)
	}
}

// userResolveID converts --user into an entitlement id. Ported from
// user.py:34,47,67 — the check is a naive substring test for '@', not email
// validation: only when user contains '@' is it resolved through the
// Identity API; anything else (including the literal string "me") is used
// as-is verbatim. This is a genuine Python quirk, not a crash, so it is kept
// as-is per policy: identities.py's 'me' special-case lives in
// resolve_identity_as_id, but user.py's own '@'-gated call site never
// reaches it for a bare "me".
func userResolveID(ctx context.Context, client *ado.Client, user string) (string, error) {
	if !strings.Contains(user, "@") {
		return user, nil
	}
	return userResolveIdentity(ctx, client, user)
}

// userIdentity is the subset of Identity (devops_sdk/v5_0/identity/models.py
// IdentityBase) this port needs to resolve an email to an id.
type userIdentity struct {
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties"`
}

// userResolveIdentity ports the email/alias branch of resolve_identity
// (common/identities.py:49-84): identity_filter.find('@') > 0 is always
// true here (userResolveID only calls this when '@' is present), so it
// always searches searchFilter=General first, falling back to
// DirectoryAlias when General finds nothing.
//
// Deviation: Python additionally disambiguates multiple matches by an extra
// connectionData round trip, comparing each candidate's Domain property
// against the caller's own identity. A literal-email search returning more
// than one identity is not a realistic case in practice (email is meant to
// be unique), so this port skips that extra call and surfaces the same
// "multiple identities found" error directly rather than attempting the
// (rare) domain-preference match — see report deviations.
func userResolveIdentity(ctx context.Context, client *ado.Client, filter string) (string, error) {
	identities, err := userReadIdentities(ctx, client, "General", filter)
	if err != nil {
		return "", err
	}
	if len(identities) == 0 {
		identities, err = userReadIdentities(ctx, client, "DirectoryAlias", filter)
		if err != nil {
			return "", err
		}
	}

	switch len(identities) {
	case 0:
		return "", fmt.Errorf("Could not resolve identity: %s", filter)
	case 1:
		return identities[0].ID, nil
	default:
		return "", fmt.Errorf("There are multiple identities found for %q Please provide a more specific identifier for this identity.", filter)
	}
}

// userReadIdentities calls the Identity API's ReadIdentities operation
// (devops_sdk/v5_0/identity/identity_client.py:148-184,
// location_id='28010c54-d0c0-4c89-a5b0-1c9e188b9fb7'), which lives on the
// vssps resource area like Graph.
func userReadIdentities(ctx context.Context, client *ado.Client, searchFilter, filterValue string) ([]userIdentity, error) {
	var identities []userIdentity
	if err := client.List(ctx, ado.Request{
		Host:       "vssps",
		Path:       "identities",
		APIVersion: "5.0",
		Query: url.Values{
			"searchFilter": {searchFilter},
			"filterValue":  {filterValue},
		},
	}, &identities); err != nil {
		return nil, fmt.Errorf("failed to resolve identity %q: %w", filterValue, err)
	}
	return identities, nil
}
