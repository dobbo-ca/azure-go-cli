package lock

import (
  "fmt"
  "regexp"
  "strings"
)

// lockIDParts is the scope carried by a lock resource ID.
type lockIDParts struct {
  ResourceGroup string
  Namespace     string
  Parent        string
  ResourceType  string
  ResourceName  string
  LockName      string
}

// lockIDRe matches a management lock ID at any of the three scopes.
//
// A resource-scoped lock ID contains TWO /providers/ segments — the locked
// resource's, then Microsoft.Authorization's — which is why the repo's
// resource.ParseResourceID cannot be reused here.
//
// Everything after the subscription ID is optional, so subscription-scoped IDs
// match with only LockName populated. There is deliberately no trailing `$`:
// azure-cli uses re.match(), which tolerates trailing garbage, and
// FindStringSubmatch matches that behavior.
var lockIDRe = regexp.MustCompile(
  `^/subscriptions/[^/]*` +
    `(?:/resource[gG]roups/([^/]*)` +
    `(?:/providers/([^/]*)` +
    `(?:/(.*))?/([^/]*)/([^/]*))?)?` +
    `/providers/Microsoft\.Authorization/locks/([^/]*)`,
)

func parseLockID(id string) (lockIDParts, error) {
  m := lockIDRe.FindStringSubmatch(id)
  if m == nil {
    return lockIDParts{}, fmt.Errorf("invalid lock ID: %s", id)
  }
  p := lockIDParts{
    ResourceGroup: m[1],
    Namespace:     m[2],
    Parent:        strings.Trim(m[3], "/"),
    ResourceType:  m[4],
    ResourceName:  m[5],
    LockName:      m[6],
  }
  if p.LockName == "" {
    return lockIDParts{}, fmt.Errorf("invalid lock ID, no lock name: %s", id)
  }
  // A namespace with no resource name is a malformed resource scope. azure-cli
  // crashes here with an unhandled AttributeError; return a real error.
  if p.Namespace != "" && p.ResourceName == "" {
    return lockIDParts{}, fmt.Errorf("invalid lock ID, no resource name: %s", id)
  }
  return p, nil
}
