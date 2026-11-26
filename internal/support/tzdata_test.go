package support

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWindowsZone(t *testing.T) {
	for iana, want := range map[string]string{
		"America/Los_Angeles": "Pacific Standard Time",
		"America/Vancouver":   "British Columbia Standard Time",
		"Etc/UTC":             "UTC",
	} {
		got, ok := windowsZone(iana)
		require.True(t, ok, iana)
		require.Equal(t, want, got, iana)
	}
	got, ok := windowsZone("Mars/Base")
	require.False(t, ok)
	require.Empty(t, got)
}

func TestWindowsZoneAliases(t *testing.T) {
	for iana, want := range map[string]string{
		"Asia/Kolkata": "India Standard Time",
		"Europe/Kyiv":  "FLE Standard Time",
		"UTC":          "UTC",
	} {
		got, ok := windowsZone(iana)
		require.True(t, ok, iana)
		require.Equal(t, want, got, iana)
	}
}

func TestWindowsZoneNames(t *testing.T) {
	names := windowsZoneNames()
	require.Contains(t, names, "Pacific Standard Time")
	require.True(t, sort.StringsAreSorted(names))
	seen := map[string]bool{}
	for _, n := range names {
		require.False(t, seen[n], n)
		seen[n] = true
	}
}

func TestIANAFromTZ(t *testing.T) {
	require.Equal(t, "Europe/Paris", ianaFromTZ(":Europe/Paris", ""))
	require.Equal(t, "America/Vancouver", ianaFromTZ("", "/var/db/timezone/zoneinfo/America/Vancouver"))
	require.Equal(t, "Asia/Kolkata", ianaFromTZ("", "/usr/share/zoneinfo/Asia/Kolkata"))
	require.Equal(t, "", ianaFromTZ("", ""))
	require.Equal(t, "", ianaFromTZ("Local", ""))
}
