package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	CredentialsFileName = "credentials"
)

// FileCache implements a file-based token cache
type FileCache struct {
	mu     sync.RWMutex
	tokens map[string]*CachedToken
}

// CachedToken represents a cached access token
type CachedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresOn    time.Time `json:"expires_on"`
	Scopes       []string  `json:"scopes"`
}

// NewFileCache creates a new file-based cache
func NewFileCache() (*FileCache, error) {
	fc := &FileCache{
		tokens: make(map[string]*CachedToken),
	}

	// Load existing tokens from file
	if err := fc.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return fc, nil
}

// GetToken retrieves a token from the cache
func (fc *FileCache) GetToken(scopes []string) (*CachedToken, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	key := scopesToKey(scopes)
	token, ok := fc.tokens[key]
	if !ok {
		return nil, false
	}

	// Check if token is expired (with 5 minute buffer)
	if time.Now().Add(5 * time.Minute).After(token.ExpiresOn) {
		return nil, false
	}

	return token, true
}

// SetToken stores a token in the cache
func (fc *FileCache) SetToken(token *CachedToken) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	key := scopesToKey(token.Scopes)
	fc.tokens[key] = token

	return fc.save()
}

// Clear removes all tokens from the cache
func (fc *FileCache) Clear() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.tokens = make(map[string]*CachedToken)
	return fc.save()
}

func (fc *FileCache) load() error {
	path, err := getCredentialsPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &fc.tokens)
}

func (fc *FileCache) save() error {
	path, err := getCredentialsPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(fc.tokens, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func getCredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".azure", CredentialsFileName), nil
}

func scopesToKey(scopes []string) string {
	if len(scopes) == 0 {
		return "default"
	}
	return scopes[0] // Use first scope as key
}

// FileCachedCredential wraps a credential with file-based caching
type FileCachedCredential struct {
	inner azcore.TokenCredential
	cache *FileCache
}

// NewFileCachedCredential creates a credential with file-based caching
func NewFileCachedCredential(inner azcore.TokenCredential) (*FileCachedCredential, error) {
	cache, err := NewFileCache()
	if err != nil {
		return nil, err
	}

	return &FileCachedCredential{
		inner: inner,
		cache: cache,
	}, nil
}

// GetToken implements the TokenCredential interface
func (fc *FileCachedCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// Try to get from cache first
	if cachedToken, ok := fc.cache.GetToken(opts.Scopes); ok {
		return azcore.AccessToken{
			Token:     cachedToken.AccessToken,
			ExpiresOn: cachedToken.ExpiresOn,
		}, nil
	}

	// Get fresh token from inner credential
	token, err := fc.inner.GetToken(ctx, opts)
	if err != nil {
		return azcore.AccessToken{}, err
	}

	// Cache the token
	cachedToken := &CachedToken{
		AccessToken: token.Token,
		ExpiresOn:   token.ExpiresOn,
		Scopes:      opts.Scopes,
	}

	if err := fc.cache.SetToken(cachedToken); err != nil {
		// Log error but don't fail - token is still valid
		fmt.Fprintf(os.Stderr, "Warning: failed to cache token: %v\n", err)
	}

	return token, nil
}

// Authenticate wraps the underlying credential's Authenticate method if available
func (fc *FileCachedCredential) Authenticate(ctx context.Context, opts *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	// Check if inner credential supports Authenticate
	if authCred, ok := fc.inner.(interface {
		Authenticate(context.Context, *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error)
	}); ok {
		return authCred.Authenticate(ctx, opts)
	}

	return azidentity.AuthenticationRecord{}, fmt.Errorf("inner credential does not support Authenticate")
}
