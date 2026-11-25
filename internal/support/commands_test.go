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
