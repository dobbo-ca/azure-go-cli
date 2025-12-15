package azure

import (
	"context"
	"os"
	"path/filepath"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// GetSharedMSALCache returns a shared file-based MSAL cache
// This ensures all credentials (base and tenant-specific) use the same token cache
func GetSharedMSALCache() (cache.ExportReplace, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(home, ".azure")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, err
	}

	cacheFile := filepath.Join(cacheDir, "msal_token_cache.json")

	return &fileMSALCache{path: cacheFile}, nil
}

// fileMSALCache implements cache.ExportReplace interface for file-based token storage
type fileMSALCache struct {
	path string
}

func (f *fileMSALCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet, that's OK
		}
		return err
	}

	return cache.Unmarshal(data)
}

func (f *fileMSALCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	data, err := cache.Marshal()
	if err != nil {
		return err
	}

	return os.WriteFile(f.path, data, 0600)
}
