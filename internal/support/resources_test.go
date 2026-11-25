package support

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceTypeFilter(t *testing.T) {
	require.Equal(t, "resourceType eq 'Microsoft.Compute/virtualMachines'", resourceTypeFilter([]string{"Microsoft.Compute/virtualMachines"}))
	require.Equal(t, "resourceType eq 'A' or resourceType eq 'B'", resourceTypeFilter([]string{"A", "B"}))
	require.Equal(t, "resourceType eq 'a''b'", resourceTypeFilter([]string{"a'b"}))
}
