package azure

// GetInteractiveBrowserCredentialWithCache creates an MSAL-based credential
// This uses the same flow as the official Azure CLI - opens browser for authentication
// Uses shared MSAL file cache so tenant-specific credentials can access the same tokens
func GetInteractiveBrowserCredentialWithCache() (*MSALInteractiveCredential, error) {
	// Use MSAL directly to ensure cache sharing
	return NewMSALInteractiveCredential()
}
