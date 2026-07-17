package lock

import (
  "strings"

  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armlocks"
)

// lockRecord is the azure-cli-shaped, flattened view of a management lock.
//
// ARM marks ManagementLockProperties with x-ms-client-flatten, so azure-cli
// prints level/notes/owners at the top level. The Go SDK keeps them nested
// under Properties, so we flatten here; without it every --query expression
// written for azure-cli (e.g. "[].level") breaks.
//
// Fields are declared in alphabetical order deliberately: knack emits JSON
// with sort_keys=True, and encoding/json emits declaration order, so this is
// what makes our key order match azure-cli's byte for byte.
//
// SystemData is intentionally absent. azure-cli pins api-version 2016-09-01,
// which has no such field; armlocks is generated against 2020-05-01, which
// does. Emitting it would diverge from azure-cli.
type lockRecord struct {
  ID            string      `json:"id"`
  Level         string      `json:"level"`
  Name          string      `json:"name"`
  Notes         *string     `json:"notes"`
  Owners        []lockOwner `json:"owners"`
  ResourceGroup string      `json:"resourceGroup,omitempty"`
  Type          string      `json:"type"`
}

type lockOwner struct {
  ApplicationID string `json:"applicationId"`
}

// resourceGroupFromID mirrors azure-cli's global _resource_group_transform,
// which injects resourceGroup into every result whose ID carries a resource
// group. It fires before output formatting, so the field shows up in JSON as
// well as table output, and is absent for subscription-scoped locks.
func resourceGroupFromID(id string) string {
  parts := strings.Split(id, "/")
  if len(parts) < 5 || !strings.EqualFold(parts[3], "resourcegroups") {
    return ""
  }
  return parts[4]
}

func toLockRecord(o *armlocks.ManagementLockObject) lockRecord {
  rec := lockRecord{}
  if o == nil {
    return rec
  }
  if o.ID != nil {
    rec.ID = *o.ID
    rec.ResourceGroup = resourceGroupFromID(*o.ID)
  }
  if o.Name != nil {
    rec.Name = *o.Name
  }
  if o.Type != nil {
    rec.Type = *o.Type
  }
  if p := o.Properties; p != nil {
    if p.Level != nil {
      rec.Level = string(*p.Level)
    }
    rec.Notes = p.Notes
    for _, ow := range p.Owners {
      if ow == nil {
        continue
      }
      owner := lockOwner{}
      if ow.ApplicationID != nil {
        owner.ApplicationID = *ow.ApplicationID
      }
      rec.Owners = append(rec.Owners, owner)
    }
  }
  return rec
}

func toLockRecords(objs []*armlocks.ManagementLockObject) []lockRecord {
  recs := make([]lockRecord, 0, len(objs))
  for _, o := range objs {
    recs = append(recs, toLockRecord(o))
  }
  return recs
}
