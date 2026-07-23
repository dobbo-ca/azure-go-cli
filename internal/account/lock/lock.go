package lock

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// LockInfo is the trimmed, script-friendly view of a management lock.
type LockInfo struct {
	Name  string `json:"name"`
	Level string `json:"level"`
	Notes string `json:"notes"`
	ID    string `json:"id"`
}

// parseLockLevel maps a user-supplied lock type to the SDK LockLevel.
// Accepted values (case-insensitive): CanNotDelete, ReadOnly.
func parseLockLevel(s string) (armlocks.LockLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "cannotdelete":
		return armlocks.LockLevelCanNotDelete, nil
	case "readonly":
		return armlocks.LockLevelReadOnly, nil
	default:
		return "", fmt.Errorf("invalid lock type %q (allowed: CanNotDelete, ReadOnly)", s)
	}
}

// lockToInfo flattens a ManagementLockObject into LockInfo.
func lockToInfo(l *armlocks.ManagementLockObject) LockInfo {
	info := LockInfo{
		Name: azure.GetStringValue(l.Name),
		ID:   azure.GetStringValue(l.ID),
	}
	if l.Properties != nil {
		if l.Properties.Level != nil {
			info.Level = string(*l.Properties.Level)
		}
		info.Notes = azure.GetStringValue(l.Properties.Notes)
	}
	return info
}

// newClient builds a subscription-scoped management-locks client using the
// default subscription and cached credentials.
func newClient() (*armlocks.ManagementLocksClient, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}

	subID, err := config.GetDefaultSubscription()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	client, err := armlocks.NewManagementLocksClient(subID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create locks client: %w", err)
	}
	return client, nil
}
