package devops

import (
	"context"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
)

// securityProjectID is services.py's get_project_id_from_name: a project
// that is already a GUID passes through unchanged, otherwise it is resolved
// via a GET on the project by name.
func securityProjectID(ctx context.Context, client *ado.Client, project string) (string, error) {
	if ado.IsUUID(project) {
		return project, nil
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := client.Do(ctx, ado.Request{
		Path:       "projects/" + url.PathEscape(project),
		APIVersion: "5.0",
	}, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// securityDescriptorFromStorageKey is security_group.py's
// get_descriptor_from_storage_key / graph_client.get_descriptor: resolve any
// storage key (a project id or an identity id) to its graph descriptor.
func securityDescriptorFromStorageKey(ctx context.Context, client *ado.Client, storageKey string) (string, error) {
	var out struct {
		Value string `json:"value"`
	}
	if err := client.Do(ctx, ado.Request{
		Host:       "vssps",
		Path:       "Graph/Descriptors/" + url.PathEscape(storageKey),
		APIVersion: "5.0-preview.1",
	}, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// securityScopeDescriptor resolves a project name/id to its graph scope
// descriptor: the two-step project-id-then-descriptor lookup shared by
// `security group list` and `security group create` (security_group.py:45-47,
// 90-93).
func securityScopeDescriptor(ctx context.Context, client *ado.Client, project string) (string, error) {
	projectID, err := securityProjectID(ctx, client, project)
	if err != nil {
		return "", err
	}
	return securityDescriptorFromStorageKey(ctx, client, projectID)
}
