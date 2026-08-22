//go:build windows

package azure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/browser"
)

// interactiveOpenURL opens the interactive sign-in URL in Microsoft Edge so
// the browser carries the device's Primary Refresh Token. Conditional Access
// policies that require a PRT (AADSTS53003) fail when the URL opens in a
// browser other than Edge. Falls back to the OS default browser (via
// github.com/pkg/browser, MSAL's own default) if Edge can't be found or
// can't be launched.
var interactiveOpenURL = openURLViaEdge

func openURLViaEdge(url string) error {
	edge, err := edgePath()
	if err != nil {
		return browser.OpenURL(url)
	}
	if err := exec.Command(edge, url).Start(); err != nil {
		return browser.OpenURL(url)
	}
	return nil
}

func edgePath() (string, error) {
	if p, err := exec.LookPath("msedge.exe"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("msedge"); err == nil {
		return p, nil
	}

	var candidates []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
		dir := os.Getenv(env)
		if dir == "" {
			continue
		}
		candidates = append(candidates, filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("microsoft edge not found")
}
