//go:build windows

package msalruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// extractEmbeddedDLL writes the embedded msalruntime.dll to a per-user cache
// directory and returns its path. Windows can only load a DLL from disk, and
// the executable's own directory may be read-only (Program Files, via the
// MSI), so the copy lands under %LOCALAPPDATA% instead.
//
// The directory is named for the DLL's content hash, which makes upgrades and
// downgrades self-sorting: a different build extracts to a different path
// rather than racing to overwrite one in use by another running process.
func extractEmbeddedDLL() (string, error) {
	if len(embeddedDLL) == 0 {
		return "", fmt.Errorf("%w: no embedded broker for this architecture", ErrNotAvailable)
	}

	sum := sha256.Sum256(embeddedDLL)
	cache, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	dir := filepath.Join(cache, "az-go", "msalruntime", hex.EncodeToString(sum[:8]))
	path := filepath.Join(dir, dllName)

	// Written already by an earlier run. Size is enough of a check: the path
	// is content-addressed, and a truncated write can never be renamed into
	// place below.
	if fi, err := os.Stat(path); err == nil && fi.Size() == int64(len(embeddedDLL)) {
		return path, nil
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}

	// Write to a unique temp file and rename, so a concurrent `az` never sees
	// or loads a partial DLL.
	tmp, err := os.CreateTemp(dir, "msalruntime-*.tmp")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(embeddedDLL); err != nil {
		tmp.Close()
		return "", fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		// Rename onto an existing file fails on Windows, which is exactly what
		// happens when another process won the race. Its copy is as good as
		// ours, so use it.
		if fi, statErr := os.Stat(path); statErr == nil && fi.Size() == int64(len(embeddedDLL)) {
			return path, nil
		}
		return "", fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	return path, nil
}
