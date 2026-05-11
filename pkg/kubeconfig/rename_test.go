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

const sampleAdminKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.hcp.eastus.azmk8s.io:443
    certificate-authority-data: AAAA
  name: proscia-prod-usw2-k8s-20251209
contexts:
- context:
    cluster: proscia-prod-usw2-k8s-20251209
    user: clusterAdmin_proscia-prod-usw2-k8s-20251209
  name: proscia-prod-usw2-k8s-20251209-admin
current-context: proscia-prod-usw2-k8s-20251209-admin
users:
- name: clusterAdmin_proscia-prod-usw2-k8s-20251209
  user:
    token: redacted
`

func TestRenameByRegex_CaptureGroup(t *testing.T) {
	pattern := regexp.MustCompile(`^proscia-(.+)$`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "acme-$1")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "acme-prod-usw2-k8s-20251209") {
		t.Errorf("expected capture-group replacement to produce acme-prod-usw2-k8s-20251209\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "clusterUser_acme-prod-usw2-k8s-20251209") {
		t.Errorf("expected user prefix preserved with renamed suffix\n--- got ---\n%s", got)
	}
}

func TestRenameByRegex_AnchoredPattern(t *testing.T) {
	// Anchored pattern matches the bare cluster name only. It must still
	// propagate to user/context fields that contain that name as a substring.
	pattern := regexp.MustCompile(`^proscia-prod-usw2-k8s-20251209$`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "mycluster")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	mustContain := []string{
		"name: mycluster",
		"cluster: mycluster",
		"user: clusterUser_mycluster",
		"current-context: mycluster",
		"name: clusterUser_mycluster",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("anchored pattern: expected %q\n--- got ---\n%s", s, got)
		}
	}
}

func TestRenameByRegex_AdminPrefixPreserved(t *testing.T) {
	pattern := regexp.MustCompile(`proscia`)
	out, err := RenameByRegex([]byte(sampleAdminKubeconfig), pattern, "acme")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "clusterAdmin_acme-prod-usw2-k8s-20251209") {
		t.Errorf("expected clusterAdmin_ prefix preserved\n--- got ---\n%s", got)
	}
	if strings.Contains(got, "proscia") {
		t.Errorf("expected all 'proscia' replaced\n--- got ---\n%s", got)
	}
}

func TestRenameByRegex_NoMatchReturnsSemanticEquivalent(t *testing.T) {
	pattern := regexp.MustCompile(`nonexistent`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "whatever")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	if !strings.Contains(string(out), "proscia-prod-usw2-k8s-20251209") {
		t.Errorf("expected original cluster name unchanged when regex does not match")
	}
}

func TestRenameByRegex_EmptyReplacement(t *testing.T) {
	pattern := regexp.MustCompile(`^proscia-`)
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), pattern, "")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "name: prod-usw2-k8s-20251209") {
		t.Errorf("expected 'proscia-' prefix stripped\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "clusterUser_prod-usw2-k8s-20251209") {
		t.Errorf("expected user prefix preserved with stripped cluster name\n--- got ---\n%s", got)
	}
}

func TestRenameByRegex_NilPatternIsNoop(t *testing.T) {
	out, err := RenameByRegex([]byte(sampleUserKubeconfig), nil, "ignored")
	if err != nil {
		t.Fatalf("RenameByRegex returned error: %v", err)
	}
	if string(out) != sampleUserKubeconfig {
		t.Errorf("nil pattern should return input unchanged")
	}
}
