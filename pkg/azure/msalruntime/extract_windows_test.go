//go:build windows

package msalruntime

import (
	"os"
	"path/filepath"
	"testing"
)

// Only runnable on Windows, which is also the only place the code runs.
func TestExtractEmbeddedDLL(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())

	path, err := extractEmbeddedDLL()
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if fi.Size() != int64(len(embeddedDLL)) {
		t.Fatalf("wrote %d bytes, embedded is %d", fi.Size(), len(embeddedDLL))
	}

	// Second call must reuse the extracted copy rather than rewrite it.
	again, err := extractEmbeddedDLL()
	if err != nil {
		t.Fatalf("second extract: %v", err)
	}
	if again != path {
		t.Fatalf("second call returned %s, want %s", again, path)
	}

	// No temp files left behind by either call.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != dllName {
			t.Errorf("leftover file %s", e.Name())
		}
	}
}
