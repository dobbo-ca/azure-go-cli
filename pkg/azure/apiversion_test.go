package azure

import "testing"

func TestSelectLatestAPIVersion(t *testing.T) {
  versions := []string{
    "2021-01-01",
    "2022-06-01-preview",
    "2023-04-01",
    "2024-01-01-preview",
  }

  t.Run("stable only", func(t *testing.T) {
    got, err := selectLatestAPIVersion(versions, false)
    if err != nil {
      t.Fatal(err)
    }
    if got != "2023-04-01" {
      t.Errorf("got %s want 2023-04-01", got)
    }
  })

  t.Run("include preview", func(t *testing.T) {
    got, err := selectLatestAPIVersion(versions, true)
    if err != nil {
      t.Fatal(err)
    }
    if got != "2024-01-01-preview" {
      t.Errorf("got %s want 2024-01-01-preview", got)
    }
  })

  t.Run("empty", func(t *testing.T) {
    if _, err := selectLatestAPIVersion(nil, false); err == nil {
      t.Error("expected error")
    }
  })

  t.Run("only preview, stable requested", func(t *testing.T) {
    if _, err := selectLatestAPIVersion([]string{"2024-01-01-preview"}, false); err == nil {
      t.Error("expected error when no stable version")
    }
  })
}
