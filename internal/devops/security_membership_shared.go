package devops

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// securityLookupSubjects is graph_client.lookup_subjects: batch-resolve
// descriptors to their GraphSubject (display name, principal name, email,
// subject kind). Returns a descriptor -> subject map, same shape as Python's
// {GraphSubject} return type.
func securityLookupSubjects(ctx context.Context, client *ado.Client, lookupKeys []map[string]string) (map[string]map[string]any, error) {
	keys := make([]map[string]string, len(lookupKeys))
	copy(keys, lookupKeys)

	var subjects map[string]map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Host:       "vssps",
		Path:       "Graph/SubjectLookup",
		APIVersion: "5.0-preview.1",
		Body:       map[string]any{"lookupKeys": keys},
	}, &subjects); err != nil {
		return nil, err
	}
	return subjects, nil
}
