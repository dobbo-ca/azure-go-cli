package credplugin

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("loadFixture(%s): %v", name, err)
	}
	return data
}

func TestConvert_LegacyAzureAuthProvider(t *testing.T) {
	in := loadFixture(t, "legacy_azure_input.yaml")
	want := loadFixture(t, "legacy_azure_expected.yaml")

	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !changed {
		t.Fatal("changed=false, want true")
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestConvert_ExistingKubeloginExec(t *testing.T) {
	in := loadFixture(t, "kubelogin_exec_input.yaml")
	want := loadFixture(t, "kubelogin_exec_expected.yaml")

	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !changed {
		t.Fatal("changed=false, want true")
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
