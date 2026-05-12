package aks

import (
	"os"
	"strings"
	"testing"
)

func TestKubeconfigPinsAZSession(t *testing.T) {
	t.Setenv("AZ_SESSION", "asdf")
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "name: AZ_SESSION") || !strings.Contains(got, `value: "asdf"`) {
		t.Fatalf("AZ_SESSION not pinned in kubeconfig:\n%s", got)
	}
}

func TestKubeconfigOmitsAZSessionWhenUnset(t *testing.T) {
	t.Setenv("AZ_SESSION", "")
	tmp := t.TempDir() + "/config"
	if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "AZ_SESSION") {
		t.Fatalf("AZ_SESSION should be absent when env var unset:\n%s", got)
	}
}

func TestWriteKubeconfig_EffectiveNameRenamesAllPositions(t *testing.T) {
	tmp := t.TempDir() + "/config"
	// Pretend the caller already applied a regex transform: pass the renamed
	// name to WriteKubeconfig. Every position in the template must use it.
	if err := WriteKubeconfig(tmp, "acme-prod-usw2-k8s-20251209", "myfqdn", 12345); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	mustContain := []string{
		"name: acme-prod-usw2-k8s-20251209",
		"cluster: acme-prod-usw2-k8s-20251209",
		"user: clusterUser_acme-prod-usw2-k8s-20251209",
		"current-context: acme-prod-usw2-k8s-20251209",
		"- name: clusterUser_acme-prod-usw2-k8s-20251209",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("expected %q in kubeconfig:\n%s", s, got)
		}
	}
}
