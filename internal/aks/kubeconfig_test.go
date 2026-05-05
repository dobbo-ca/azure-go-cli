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
