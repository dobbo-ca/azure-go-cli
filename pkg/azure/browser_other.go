//go:build !windows

package azure

// interactiveOpenURL is nil on non-Windows platforms, leaving MSAL's default
// browser-opening behavior unchanged.
var interactiveOpenURL func(url string) error
