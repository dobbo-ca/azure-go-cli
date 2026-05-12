package kubeconfig

import (
	"strings"
	"testing"
)

func TestUpdateContext_RenamesAllIdentifiers(t *testing.T) {
	out, err := UpdateContext([]byte(sampleUserKubeconfig), "myalias")
	if err != nil {
		t.Fatalf("UpdateContext returned error: %v", err)
	}
	got := string(out)

	mustContain := []string{
		"name: myalias",
		"cluster: myalias",
		"user: clusterUser_myalias",
		"current-context: myalias",
		"name: clusterUser_myalias",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("UpdateContext: expected %q in output\n--- got ---\n%s", s, got)
		}
	}
	if strings.Contains(got, "proscia-prod-usw2-k8s-20251209") {
		t.Errorf("UpdateContext: expected old cluster name to be fully replaced\n--- got ---\n%s", got)
	}
}

func TestUpdateContext_NoClustersReturnsUnchanged(t *testing.T) {
	input := []byte("apiVersion: v1\nkind: Config\nclusters: []\n")
	out, err := UpdateContext(input, "myalias")
	if err != nil {
		t.Fatalf("UpdateContext returned error: %v", err)
	}
	if string(out) != string(input) {
		t.Errorf("expected empty-clusters input to be returned unchanged")
	}
}
