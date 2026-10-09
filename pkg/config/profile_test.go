package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStripsBOM(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AZ_SESSION", "")
	os.MkdirAll(filepath.Join(home, ConfigDir), 0700)
	data := append([]byte("\xef\xbb\xbf"), `{"subscriptions":[{"id":"s1"}]}`...)
	if err := os.WriteFile(filepath.Join(home, ConfigDir, ConfigFile), data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load()
	if err != nil || len(p.Subscriptions) != 1 || p.Subscriptions[0].ID != "s1" {
		t.Fatalf("got %+v, %v", p, err)
	}
}
