package support

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestTuiAllowed(t *testing.T) {
	require.True(t, tuiAllowed(false, true, true))
	require.False(t, tuiAllowed(true, true, true))
	require.False(t, tuiAllowed(false, false, true))
	require.False(t, tuiAllowed(false, true, false))
}

// parityRows: command path, then flags with a leading * meaning required.
var parityRows = []struct{ path, flags string }{
	{"services show", "*service-name"},
	{"services problem-classifications list", "*service-name"},
	{"services problem-classifications show", "*service-name *problem-classification-name"},
	{"in-subscription tickets create", "*ticket-name *advanced-diagnostic-consent *description *problem-classification *severity *title start-time require-24-by-7-response secondary-consent enrollment-id technical-resource *contact-country *contact-first-name *contact-last-name *contact-method *contact-language *contact-timezone *contact-email contact-phone-number contact-additional-emails file-workspace quota-change-subtype quota-change-version quota-change-requests"},
	{"in-subscription tickets update", "*ticket-name advanced-diagnostic-consent severity status secondary-consent contact-country contact-first-name contact-last-name contact-method contact-language contact-timezone contact-email contact-phone-number contact-additional-emails"},
	{"in-subscription tickets list", "filter max-items"},
	{"in-subscription tickets show", "*ticket-name"},
	{"no-subscription tickets create", "*ticket-name *advanced-diagnostic-consent *description *problem-classification *severity *title start-time require-24-by-7-response secondary-consent enrollment-id technical-resource *contact-country *contact-first-name *contact-last-name *contact-method *contact-language *contact-timezone *contact-email contact-phone-number contact-additional-emails file-workspace-name"},
	{"no-subscription tickets update", "*ticket-name advanced-diagnostic-consent severity status secondary-consent contact-country contact-first-name contact-last-name contact-method contact-language contact-timezone contact-email contact-phone-number contact-additional-emails"},
	{"no-subscription tickets list", "filter max-items"},
	{"no-subscription tickets show", "*ticket-name"},
	{"in-subscription communication create", "*ticket-name *communication-name *communication-subject *communication-body communication-sender"},
	{"in-subscription communication list", "*ticket-name filter max-items"},
	{"in-subscription communication show", "*ticket-name *communication-name"},
	{"in-subscription chat-transcript list", "*ticket-name max-items"},
	{"in-subscription chat-transcript show", "*ticket-name *chat-transcript-name"},
	{"in-subscription file upload", "*file-workspace-name *file-path"},
	{"in-subscription file list", "*file-workspace-name max-items"},
	{"in-subscription file show", "*file-workspace-name *file-name"},
	{"in-subscription file-workspace create", "*file-workspace-name"},
	{"in-subscription file-workspace show", "*file-workspace-name"},
	{"no-subscription communication create", "*ticket-name *communication-name *communication-subject *communication-body communication-sender"},
	{"no-subscription communication list", "*ticket-name filter max-items"},
	{"no-subscription communication show", "*ticket-name *communication-name"},
	{"no-subscription chat-transcript list", "*ticket-name max-items"},
	{"no-subscription chat-transcript show", "*ticket-name *chat-transcript-name"},
	{"no-subscription file upload", "*file-workspace-name *file-path"},
	{"no-subscription file list", "*file-workspace-name max-items"},
	{"no-subscription file show", "*file-workspace-name *file-name"},
	{"no-subscription file-workspace create", "*file-workspace-name"},
	{"no-subscription file-workspace show", "*file-workspace-name"},
}

func TestFlagParity(t *testing.T) {
	for _, row := range parityRows {
		cmd, _, err := NewSupportCommand().Find(strings.Fields(row.path))
		require.NoError(t, err, row.path)
		for _, spec := range strings.Fields(row.flags) {
			name, required := strings.TrimPrefix(spec, "*"), strings.HasPrefix(spec, "*")
			f := cmd.Flags().Lookup(name)
			require.NotNil(t, f, "%s --%s", row.path, name)
			require.Equal(t, required, f.Annotations[cobra.BashCompOneRequiredFlag] != nil, "%s --%s", row.path, name)
		}
	}
}

func TestSupportNoSubcommandNonTTYPrintsHelp(t *testing.T) {
	cmd := NewSupportCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--no-input"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "in-subscription")
}

func TestNoSubscriptionCreateHasNoQuotaFlags(t *testing.T) {
	cmd, _, err := NewSupportCommand().Find([]string{"no-subscription", "tickets", "create"})
	require.NoError(t, err)
	require.Nil(t, cmd.Flags().Lookup("quota-change-subtype"))
}
