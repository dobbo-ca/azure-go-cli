package kubeconfig

import (
	"regexp"
	"strings"
	"testing"
)

const sampleUserKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io:443
    certificate-authority-data: AAAA
  name: proscia-prod-usw2-k8s-20251209
contexts:
- context:
    cluster: proscia-prod-usw2-k8s-20251209
    user: clusterUser_proscia-prod-usw2-k8s-20251209
  name: proscia-prod-usw2-k8s-20251209
current-context: proscia-prod-usw2-k8s-20251209
users:
- name: clusterUser_proscia-prod-usw2-k8s-20251209
  user:
    token: redacted
`

func TestRenameByRegex_BasicSubstring(t *testing.T) {
	pattern := regexp.MustCompile(`proscia`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "acme")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)

	mustContain := []string{
		"name: acme-prod-usw2-k8s-20251209",
		"cluster: acme-prod-usw2-k8s-20251209",
		"user: clusterUser_acme-prod-usw2-k8s-20251209",
		"current-context: acme-prod-usw2-k8s-20251209",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("expected output to contain %q\n--- got ---\n%s", s, got)
		}
	}
	if strings.Contains(got, "proscia") {
		t.Errorf("expected all occurrences of 'proscia' to be replaced\n--- got ---\n%s", got)
	}
}
