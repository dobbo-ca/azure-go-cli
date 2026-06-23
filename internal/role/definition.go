package role

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
)

// roleDefinitionRecord is the azure-cli-shaped, flattened view of a role
// definition emitted for json/tsv output (so --query expressions match).
type roleDefinitionRecord struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name"`
	Type             string                     `json:"type"`
	RoleName         string                     `json:"roleName"`
	RoleType         string                     `json:"roleType"`
	Description      string                     `json:"description"`
	AssignableScopes []string                   `json:"assignableScopes"`
	Permissions      []roleDefinitionPermRecord `json:"permissions"`
}

type roleDefinitionPermRecord struct {
	Actions        []string `json:"actions"`
	NotActions     []string `json:"notActions"`
	DataActions    []string `json:"dataActions"`
	NotDataActions []string `json:"notDataActions"`
}

func toDefinitionRecords(roles []*armauthorization.RoleDefinition) []roleDefinitionRecord {
	records := make([]roleDefinitionRecord, 0, len(roles))
	for _, r := range roles {
		rec := roleDefinitionRecord{
			AssignableScopes: []string{},
			Permissions:      []roleDefinitionPermRecord{},
		}
		if r.ID != nil {
			rec.ID = *r.ID
		}
		if r.Name != nil {
			rec.Name = *r.Name
		}
		if r.Type != nil {
			rec.Type = *r.Type
		}
		if p := r.Properties; p != nil {
			if p.RoleName != nil {
				rec.RoleName = *p.RoleName
			}
			if p.RoleType != nil {
				rec.RoleType = *p.RoleType
			}
			if p.Description != nil {
				rec.Description = *p.Description
			}
			for _, s := range p.AssignableScopes {
				if s != nil {
					rec.AssignableScopes = append(rec.AssignableScopes, *s)
				}
			}
			for _, perm := range p.Permissions {
				if perm == nil {
					continue
				}
				rec.Permissions = append(rec.Permissions, roleDefinitionPermRecord{
					Actions:        derefStrings(perm.Actions),
					NotActions:     derefStrings(perm.NotActions),
					DataActions:    derefStrings(perm.DataActions),
					NotDataActions: derefStrings(perm.NotDataActions),
				})
			}
		}
		records = append(records, rec)
	}
	return records
}

func derefStrings(in []*string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}
