package devops

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminBannerKey and adminSettingsAPIVersion are shared across the admin
// banner commands (dev/admin/setting.py:26-27, GLOBAL_MESSAGE_BANNERS_KEY /
// USER_SCOPE_HOST + settings_client.py's hardcoded "5.0-preview.1").
const (
	adminBannerKey          = "GlobalMessageBanners"
	adminSettingsAPIVersion = "5.0-preview.1"
)

// newAdminCommand returns the "devops admin" command group. Only "banner" is
// wired here: dev/admin/arguments.py references a "devops admin user"
// argument context, but no command group registers it anywhere in the
// Python source (dev/admin has no user.py) — it is dead code upstream, so
// there is nothing to port.
func newAdminCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage azure devops admin operations.",
	}
	cmd.AddCommand(adminBannerGroupCmd())
	return cmd
}

func adminBannerGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "banner",
		Short: "Manage banners.",
	}
	cmd.AddCommand(adminBannerListCmd())
	cmd.AddCommand(adminBannerShowCmd())
	cmd.AddCommand(adminBannerAddCmd())
	cmd.AddCommand(adminBannerUpdateCmd())
	cmd.AddCommand(adminBannerRemoveCmd())
	return cmd
}

// adminBannerColumns is transform_banner_table_output (dev/admin/_format.py:9-34):
// ID, Message, Type, Expiration Date, with the exact placeholder/default
// rules from _transform_banner_row. Columns operate on a row map that is one
// banner's entry with "id" injected (see adminPrintBanners).
var adminBannerColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Message", Value: func(row map[string]any) string {
		if v, ok := row["message"]; ok {
			return adminStr(v)
		}
		return " "
	}},
	{Header: "Type", Value: func(row map[string]any) string {
		if v, ok := row["level"]; ok {
			s := adminStr(v)
			if s != "" {
				return strings.ToUpper(s[:1]) + s[1:]
			}
			return s
		}
		return "Info"
	}},
	{Header: "Expiration Date", Value: func(row map[string]any) string {
		if v, ok := row["expirationDate"]; ok {
			return adminStr(v)
		}
		return " "
	}},
}

// adminFetchBanners performs the single GET both banner_list and
// banner_show/banner_update use to fetch every banner (banner.py:12-14,23-24,
// 88-91): GET .../Settings/Entries/host/GlobalMessageBanners. The response is
// wrapped in Azure DevOps's generic VssJsonCollectionWrapper
// ({"count":N,"value":{...}}, client.py:109-115) even though "value" here is
// an object, not an array — that's why this uses Do (with a wrapper struct),
// not List (which only unwraps array-shaped "value").
func adminFetchBanners(ctx context.Context, client *ado.Client) (map[string]map[string]any, error) {
	var wrapper struct {
		Value map[string]map[string]any `json:"value"`
	}
	if err := client.Do(ctx, ado.Request{
		Path:       "Settings/Entries/host/" + adminBannerKey,
		APIVersion: adminSettingsAPIVersion,
	}, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Value == nil {
		return map[string]map[string]any{}, nil
	}
	return wrapper.Value, nil
}

// adminPrintBanners renders entries (a dict keyed by banner id, exactly the
// shape banner_list/show/add/update return) through ado.Print. JSON/tsv/
// --query render the dict as-is, matching Python's return value byte for
// byte. Table mode needs one row per banner with an explicit "id" field —
// ado.Print's table path treats a bare map[string]any as a single row, so
// this builds that row list itself (sorted by id: encoding/json's decode
// into a Go map does not preserve the server's key order the way Python's
// dict does, so a stable order is substituted for a faithful one).
func adminPrintBanners(cmd *cobra.Command, entries map[string]map[string]any) error {

	if ado.TableMode(cmd) {
		ids := make([]string, 0, len(entries))
		for id := range entries {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		rows := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			row := map[string]any{"id": id}
			for k, v := range entries[id] {
				row[k] = v
			}
			rows = append(rows, row)
		}
		return ado.Print(cmd, rows, adminBannerColumns...)
	}

	return ado.Print(cmd, entries, adminBannerColumns...)
}

// adminStr renders a JSON-decoded scalar (always a plain string for these
// banner fields) as a table cell; nil (JSON null) renders as "".
func adminStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// adminValidateBannerType enforces the --type/-t choices ("info", "warning",
// "error", dev/admin/arguments.py:18); "" (not supplied) is valid too.
func adminValidateBannerType(t string) error {
	switch t {
	case "", "info", "warning", "error":
		return nil
	default:
		return fmt.Errorf("invalid --type %q: must be one of info, warning, error", t)
	}
}

// adminParseExpiration ports convert_date_string_to_iso8601 (dev/common/
// arguments.py:15-28) for the handful of date shapes the surface doc calls
// out ("2019-06-10 17:21:00 UTC", "2019-06-10", plain ISO 8601). Python uses
// dateutil.parser.parse, which is far more permissive than any fixed layout
// list; reproducing it exactly would need a third-party module, which the
// no-new-dependency rule forbids. ponytail: stdlib layout list covers the
// documented formats; widen it if a real --expiration value fails to parse.
// A value with no explicit timezone is localized to the machine's local zone
// (arguments.py:26-27, tzlocal()), matching Python.
func adminParseExpiration(value string) (string, error) {
	layouts := []struct {
		layout string
		hasTZ  bool
	}{
		{time.RFC3339, true},
		{"2006-01-02T15:04:05", false},
		{"2006-01-02 15:04:05 MST", true},
		{"2006-01-02 15:04:05", false},
		{"2006-01-02", false},
		{"01/02/2006", false},
	}
	for _, l := range layouts {
		var t time.Time
		var err error
		if l.hasTZ {
			t, err = time.Parse(l.layout, value)
		} else {
			t, err = time.ParseInLocation(l.layout, value, time.Local)
		}
		if err == nil {
			// Python's d.isoformat() always emits a numeric UTC offset
			// (e.g. "+00:00"), never "Z" — "-07:00" (not "Z07:00") forces
			// that in Go's time.Format too.
			return t.Format("2006-01-02T15:04:05-07:00"), nil
		}
	}
	return "", fmt.Errorf("The --expiration argument must be a valid ISO 8601 string.")
}

// adminNewUUID generates a random (v4) UUID, matching uuid.uuid4()
// (banner.py:50-51) used to auto-generate --id when omitted. No stdlib UUID
// type exists, so this is hand-rolled per RFC 4122 from crypto/rand.
func adminNewUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
