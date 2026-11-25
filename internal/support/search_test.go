package support

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRank(t *testing.T) {
	items := []string{"Virtual Machine › Cannot connect", "Storage › Cannot connect to blob", "Virtual Machine › Performance"}
	require.Equal(t, []int{0, 1, 2}, rank("", items))
	require.Equal(t, []int{0, 2}, rank("virtual", items))
	require.Equal(t, []int{0, 1, 2}, rank("virtual connect", items))
	require.Equal(t, []int{1}, rank("BLOB", items))
	require.Empty(t, rank("kubernetes", items))
}

func TestQueryFromTitle(t *testing.T) {
	require.Equal(t, "connect reboot", queryFromTitle("Cannot connect to my VM after the reboot!!"))
}
