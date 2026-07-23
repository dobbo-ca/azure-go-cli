package flexibleserver

import (
	"strings"
	"testing"
)

func TestBuildConnectionStrings(t *testing.T) {
	cs := buildConnectionStrings("mysrv", "appdb", "adm", "secret")

	if got := cs["uri"]; got != "postgresql://adm:secret@mysrv.postgres.database.azure.com:5432/appdb?sslmode=require" {
		t.Errorf("uri = %q", got)
	}
	if !strings.Contains(cs["psql"], "host=mysrv.postgres.database.azure.com") {
		t.Errorf("psql missing FQDN host: %q", cs["psql"])
	}
	for _, k := range []string{"psql", "uri", "jdbc", "ado.net", "node.js", "python", "php", "ruby"} {
		if _, ok := cs[k]; !ok {
			t.Errorf("missing connection string for %q", k)
		}
	}
}

// A fully-qualified server name must not get the domain suffix appended twice.
func TestBuildConnectionStringsFQDN(t *testing.T) {
	cs := buildConnectionStrings("mysrv.postgres.database.azure.com", "postgres", "u", "p")
	if strings.Count(cs["uri"], ".postgres.database.azure.com") != 1 {
		t.Errorf("domain appended twice: %q", cs["uri"])
	}
}
