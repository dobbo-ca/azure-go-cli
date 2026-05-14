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

func TestConvert_MultiUser_OnlyAADUserRewritten(t *testing.T) {
	in := loadFixture(t, "multi_user_input.yaml")
	want := loadFixture(t, "multi_user_expected.yaml")
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

func TestConvert_AdminOnly_Unchanged(t *testing.T) {
	in := loadFixture(t, "admin_only.yaml")
	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if changed {
		t.Errorf("changed=true, want false")
	}
	if string(got) != string(in) {
		t.Errorf("admin-only kubeconfig should be returned byte-for-byte unchanged when changed=false")
	}
}

func TestConvert_AlreadyConverted_Unchanged(t *testing.T) {
	in := loadFixture(t, "already_converted.yaml")
	got, changed, err := Convert(in, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if changed {
		t.Errorf("changed=true, want false")
	}
	if string(got) != string(in) {
		t.Errorf("already-converted kubeconfig should be returned byte-for-byte unchanged")
	}
}

func TestConvert_MalformedYAML(t *testing.T) {
	_, _, err := Convert([]byte("not: valid: yaml: ::"), ConvertOptions{})
	if err == nil {
		t.Fatal("want parse error, got nil")
	}
}
