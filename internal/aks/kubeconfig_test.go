package aks

import (
  "os"
  "strings"
  "testing"
)

func TestKubeconfigPinsAZSession(t *testing.T) {
  t.Setenv("AZ_SESSION", "asdf")
  tmp := t.TempDir() + "/config"
  if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, false); err != nil {
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
  if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, false); err != nil {
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
  if err := WriteKubeconfig(tmp, "acme-prod-usw2-k8s-20251209", "myfqdn", 12345, false); err != nil {
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

func TestWriteKubeconfig_UsesAzGetToken(t *testing.T) {
  tmp := t.TempDir() + "/config"
  if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, false); err != nil {
    t.Fatal(err)
  }
  data, _ := os.ReadFile(tmp)
  got := string(data)

  mustContain := []string{
    "command: az",
    "- aks",
    "- get-token",
    "- --server-id",
    "- 6dae42f8-4368-4678-94ff-3960e28e3630",
  }
  for _, s := range mustContain {
    if !strings.Contains(got, s) {
      t.Errorf("expected %q in kubeconfig:\n%s", s, got)
    }
  }
  if strings.Contains(got, "kubelogin") {
    t.Errorf("kubeconfig must not reference kubelogin:\n%s", got)
  }
}

func TestWriteKubeconfig_AbsolutePath(t *testing.T) {
  tmp := t.TempDir() + "/config"
  if err := WriteKubeconfig(tmp, "mycluster", "myfqdn", 12345, true); err != nil {
    t.Fatal(err)
  }
  exe, _ := os.Executable()
  data, _ := os.ReadFile(tmp)
  got := string(data)
  if !strings.Contains(got, "command: "+exe) {
    t.Errorf("expected absolute exe path in kubeconfig:\n%s", got)
  }
}
