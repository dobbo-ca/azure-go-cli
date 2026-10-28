package ssh

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"golang.org/x/term"
)

const hybridConnectivityAPIVersion = "2023-03-15"

// relayInfo is the relay object from the endpoint's listCredentials.
type relayInfo struct {
	NamespaceName             string `json:"namespaceName"`
	NamespaceNameSuffix       string `json:"namespaceNameSuffix"`
	HybridConnectionName      string `json:"hybridConnectionName"`
	AccessKey                 string `json:"accessKey"`
	ExpiresOn                 int64  `json:"expiresOn"`
	ServiceConfigurationToken string `json:"serviceConfigurationToken"`
}

// encode is connectivity_utils.format_relay_info_string: base64 of
// json.dumps output, with its ", " and ": " separators.
func (r *relayInfo) encode() (string, error) {
	if r == nil {
		return "", errors.New("no relay information")
	}
	str := func(v string) string {
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(v)
		return strings.TrimSuffix(b.String(), "\n")
	}
	text := fmt.Sprintf(`{"relay": {"namespaceName": %s, "namespaceNameSuffix": %s, "hybridConnectionName": %s, "accessKey": %s, "expiresOn": %d, "serviceConfigurationToken": %s}}`,
		str(r.NamespaceName), str(r.NamespaceNameSuffix), str(r.HybridConnectionName), str(r.AccessKey), r.ExpiresOn, str(r.ServiceConfigurationToken))
	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

func (s *session) endpointPath() (string, error) {
	a, err := s.armClient()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s/%s/providers/Microsoft.HybridConnectivity/endpoints/default",
		a.subscriptionID, s.resourceGroup, s.resourceType, s.vmName), nil
}

// getRelayInformation is connectivity_utils.get_relay_information.
func getRelayInformation(ctx context.Context, s *session, certLifetime float64) error {
	expiresIn := int(certLifetime)
	if expiresIn <= 0 || expiresIn > relayInfoMaxDurationSecs {
		expiresIn = relayInfoMaxDurationSecs
	}

	cred, status, body, err := listCredentials(ctx, s, expiresIn)
	switch {
	case err != nil:
		return fmt.Errorf("Unable to retrieve relay information. Failed with error: %s", err)
	case status == http.StatusNotFound:
		if err := createDefaultEndpoint(ctx, s); err != nil {
			return err
		}
	case status == http.StatusPreconditionFailed:
	case status >= 400:
		return fmt.Errorf("Unable to retrieve relay information. Failed with error: %s", httpErrorText(status, body))
	}

	refresh := cred == nil
	if !refresh {
		ok, err := checkServiceConfiguration(ctx, s)
		if err != nil {
			return err
		}
		refresh = !ok
	}
	if refresh {
		if err := createServiceConfiguration(ctx, s); err != nil {
			return err
		}
		s.newServiceConfig = true
		cred, status, body, err = listCredentials(ctx, s, expiresIn)
		if err == nil && cred == nil {
			err = errors.New(httpErrorText(status, body))
		}
		if err != nil {
			return fmt.Errorf("Unable to get relay information. Failed with error: %s", err)
		}
		if err := handleRelayConnectionDelay(ctx, "Setting up service configuration"); err != nil {
			return err
		}
	}
	s.relay = cred
	return nil
}

func listCredentials(ctx context.Context, s *session, expiresIn int) (*relayInfo, int, []byte, error) {
	path, err := s.endpointPath()
	if err != nil {
		return nil, 0, nil, err
	}
	status, body, err := s.armDo(ctx, http.MethodPost, path+"/listCredentials", hybridConnectivityAPIVersion,
		url.Values{"expiresin": {strconv.Itoa(expiresIn)}}, map[string]string{"serviceName": "SSH"})
	if err != nil || status != http.StatusOK {
		return nil, status, body, err
	}
	var resp struct {
		Relay *relayInfo `json:"relay"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, status, body, err
	}
	if resp.Relay == nil {
		return nil, status, body, errors.New("response has no relay information")
	}
	return resp.Relay, status, body, nil
}

// checkServiceConfiguration is _check_service_configuration. A failed
// lookup counts as fine, as upstream: the connection itself will tell.
func checkServiceConfiguration(ctx context.Context, s *session) (bool, error) {
	path, err := s.endpointPath()
	if err != nil {
		return true, nil
	}
	status, body, err := s.armDo(ctx, http.MethodGet, path+"/serviceConfigurations/SSH", hybridConnectivityAPIVersion, nil, nil)
	if err != nil || status != http.StatusOK {
		return true, nil
	}
	var sc struct {
		Properties struct {
			Port int `json:"port"`
		} `json:"properties"`
	}
	if json.Unmarshal(body, &sc) != nil {
		return true, nil
	}
	if s.port != "" {
		want, err := strconv.Atoi(s.port)
		if err != nil {
			return false, fmt.Errorf("invalid --port %q: %w", s.port, err)
		}
		return sc.Properties.Port == want, nil
	}
	if sc.Properties.Port != 22 {
		warnf("The Service Configuration endpoint is set to a non-default port. To either update the Service " +
			"Configuration or connect using the non-default port, please use the -Port parameter")
	}
	return true, nil
}

func createDefaultEndpoint(ctx context.Context, s *session) error {
	path, err := s.endpointPath()
	if err != nil {
		return err
	}
	status, body, err := s.armDo(ctx, http.MethodPut, path, hybridConnectivityAPIVersion, nil,
		map[string]any{"properties": map[string]string{"type": "default"}})
	if err == nil && status >= 400 {
		if status == http.StatusForbidden {
			return fmt.Errorf("Client is not authorized to create a default connectivity endpoint for '%s' in Resource Group '%s'. "+
				"This is a one-time operation that must be performed by an account with Owner or Contributor role to allow "+
				"connections to the target resource.", s.vmName, s.resourceGroup)
		}
		err = errors.New(httpErrorText(status, body))
	}
	if err != nil {
		return fmt.Errorf("Failed to create default endpoint for the target Arc Server. \nError: %s", err)
	}
	return nil
}

func createServiceConfiguration(ctx context.Context, s *session) error {
	port := s.port
	prompt := fmt.Sprintf("The port %s that you are trying to connect to is not allowed for SSH connections for this "+
		"resource. Would you like to update the current Service Configuration in the endpoint to allow connections to port %s?", port, port)
	if port == "" {
		port = "22"
		prompt = "Port 22 is not allowed for SSH connections in this resource. Would you like to update the current " +
			"Service Configuration in the endpoint to allow connections to port 22? If you would like to update the " +
			"Service Configuration to allow connections to a different port, please provide the -Port parameter or " +
			"manually set up the Service Configuration."
	}
	if !s.yes {
		ok, err := promptYN(ctx, prompt)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("SSH connection is not enabled in the target port %s.", port)
		}
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid --port %q: %w", port, err)
	}

	path, err := s.endpointPath()
	if err != nil {
		return err
	}
	status, body, err := s.armDo(ctx, http.MethodPut, path+"/serviceConfigurations/SSH", hybridConnectivityAPIVersion, nil,
		map[string]any{"properties": map[string]any{"port": portNum, "serviceName": "SSH"}})
	if err == nil && status >= 400 {
		if status == http.StatusForbidden {
			return fmt.Errorf("Client is not authorized to create or update the Service Configuration endpoint for '%s' in the "+
				"Resource Group '%s'. This is an operation that must be performed by an account with Owner or Contributor "+
				"role to allow SSH connections to the specified port %s.", s.vmName, s.resourceGroup, port)
		}
		err = errors.New(httpErrorText(status, body))
	}
	if err != nil {
		return fmt.Errorf("Failed to create service configuration to allow SSH connections to port %s on the endpoint for %s "+
			"in the Resource Group %s\nError: %s", port, s.vmName, s.resourceGroup, err)
	}
	return nil
}

// promptYN is knack's prompt_y_n: y or n, asked again until it gets one.
// Ctrl+C cancels ctx and aborts the prompt.
func promptYN(ctx context.Context, msg string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("Unable to prompt for confirmation as no tty available. Use --yes.")
	}
	type answer struct {
		ok  bool
		err error
	}
	ch := make(chan answer, 1)
	go func() {
		r := bufio.NewReader(os.Stdin)
		for {
			fmt.Printf("%s (y/n): ", msg)
			line, err := r.ReadString('\n')
			switch strings.ToLower(strings.TrimSpace(line)) {
			case "y":
				ch <- answer{ok: true}
				return
			case "n":
				ch <- answer{}
				return
			}
			if err != nil {
				ch <- answer{err: fmt.Errorf("failed to read confirmation: %w", err)}
				return
			}
		}
	}()
	select {
	case a := <-ch:
		return a.ok, a.err
	case <-ctx.Done():
		fmt.Println()
		return false, ctx.Err()
	}
}

// handleRelayConnectionDelay waits out the relay's reconnect delay, or
// the first ssh attempt fails.
func handleRelayConnectionDelay(ctx context.Context, message string) error {
	for x := 0; x <= serviceConnectionDelaySecs; x++ {
		pct := x * 100 / serviceConnectionDelaySecs
		fmt.Fprintf(os.Stderr, "\r%s: [%-20s] %3d%%", message, strings.Repeat("#", pct/5), pct)
		if err := sleepCtx(ctx, time.Second); err != nil {
			fmt.Fprintln(os.Stderr)
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "\r%s: complete%s\n", message, strings.Repeat(" ", 24))
	return nil
}

// installClientSideProxy is connectivity_utils.install_client_side_proxy.
func installClientSideProxy(ctx context.Context, proxyFolder string) (string, error) {
	goos := runtime.GOOS
	if goos != "linux" && goos != "darwin" && goos != "windows" {
		return "", fmt.Errorf("Unsuported OS: %s platform is not currently supported", goos)
	}
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" && arch != "386" {
		return "", fmt.Errorf("Unsuported architecture: %s is not currently supported", arch)
	}
	if goos == "darwin" && arch == "386" {
		return "", errors.New("Unsupported Darwin OS with 386 architecture.")
	}
	proxyName := fmt.Sprintf("sshProxy_%s_%s_%s", goos, arch, strings.ReplaceAll(clientProxyVersion, ".", "_"))
	if goos == "windows" {
		proxyName += ".exe"
	}
	installDir := proxyFolder
	if installDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		installDir = filepath.Join(home, ".clientsshproxy")
	}
	location := filepath.Join(installDir, proxyName)
	if isFile(location) {
		return location, nil
	}

	if !isDir(installDir) {
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			return "", fmt.Errorf("Failed to create client proxy directory '%s'.Error: %s", installDir, err)
		}
	} else {
		older, _ := filepath.Glob(filepath.Join(installDir, fmt.Sprintf("sshProxy_%s_%s_*", goos, arch)))
		for _, f := range older {
			if err := os.Remove(f); err != nil {
				warnf("failed to delete older version file %s", f)
			}
		}
	}
	if err := downloadProxy(ctx, installDir, proxyName, goos, arch); err != nil {
		return "", err
	}
	return location, checkProxyInstallation(installDir, proxyName)
}

// downloadProxy pulls the proxy from MCR the way oras does, but checks the
// blob's digest, which oras and upstream skip.
func downloadProxy(ctx context.Context, installDir, proxyName, goos, arch string) error {
	repo := fmt.Sprintf("https://mcr.microsoft.com/v2/%s/%s/%s/ssh-proxy", clientProxyMCRRepo, goos, arch)
	logger.Debug("Downloading Arc Connectivity Proxy from %s in Microsoft Artifact Regristy.", repo)
	fail := func(err error) error {
		return fmt.Errorf("Failed to download Arc Connectivity proxy with error %s. Please try again.", err)
	}
	unexpected := errors.New("Attempt to download Arc Connectivity Proxy returned unnexpected result. Please try again.")

	manifestBytes, err := httpGet(ctx, repo+"/manifests/"+clientProxyVersion, "application/vnd.oci.image.manifest.v1+json")
	if err != nil {
		return fail(err)
	}
	var manifest struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fail(err)
	}
	if len(manifest.Layers) != 1 {
		return unexpected
	}
	digest := manifest.Layers[0].Digest
	blob, err := httpGet(ctx, repo+"/blobs/"+digest, "")
	if err != nil {
		return fail(err)
	}
	sum := sha256.Sum256(blob)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != digest {
		return fail(fmt.Errorf("digest mismatch: got %s, want %s", got, digest))
	}

	files, err := proxyTarFiles(blob, proxyName)
	if err != nil {
		return unexpected
	}
	// The proxy goes last: its presence is what marks the install done.
	for name, data := range files {
		if name != proxyName {
			if err := writeFileAtomic(filepath.Join(installDir, name), data, 0o644); err != nil {
				return fail(err)
			}
		}
	}
	if data, ok := files[proxyName]; ok {
		if err := writeFileAtomic(filepath.Join(installDir, proxyName), data, 0o755); err != nil {
			return fail(err)
		}
	}
	return nil
}

// writeFileAtomic never leaves a partial file at path, so a failed
// download is retried instead of reused.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// proxyTarFiles is _extract_proxy_tar_files: a flat tarball holding the
// proxy binary and its two licence files, checked before anything is written.
func proxyTarFiles(blob []byte, proxyName string) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	files := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		parts := strings.Split(hdr.Name, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected tar entry %s", hdr.Name)
		}
		name := parts[1]
		switch lower := strings.ToLower(name); {
		case strings.HasPrefix(name, "sshproxy"):
			name = proxyName
		case lower != "license.txt" && lower != "thirdpartynotice.txt":
			return nil, fmt.Errorf("unexpected tar entry %s", hdr.Name)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
}

func httpGet(ctx context.Context, rawURL, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return body, nil
}

func checkProxyInstallation(installDir, proxyName string) error {
	proxyPath := filepath.Join(installDir, proxyName)
	if !isFile(proxyPath) {
		return fmt.Errorf("Failed to install required SSH Arc Connectivity Proxy. Couldn't find expected file %s. Please try again.", proxyPath)
	}
	successf("Successfuly installed SSH Connectivity Proxy file %s", proxyPath)
	for _, name := range []string{"LICENSE.txt", "ThirdPartyNotice.txt"} {
		p := filepath.Join(installDir, name)
		if isFile(p) {
			successf("Successfuly installed SSH Connectivity Proxy License file %s", p)
		} else {
			warnf("Failed to download Arc Connectivity Proxy license file %s. Clouldn't find expected file %s. This won't affect your connection.", name, p)
		}
	}
	return nil
}
