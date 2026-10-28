package ssh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// runConfig is custom.ssh_config.
func runConfig(ctx context.Context, s *session) error {
	if (s.publicKeyFile != "" || s.privateKeyFile != "") && s.credentialsFolder != "" {
		return errors.New("--keys-destination-folder can't be used in conjunction with --public-key-file/-p or --private-key-file/-i.")
	}
	if err := assertArgs(s.resourceGroup, s.vmName, s.ip, s.resourceType, s.certFile, s.localUser); err != nil {
		return err
	}
	s.makePathsAbsolute()

	rt, err := decideResourceType(ctx, s)
	if err != nil {
		return err
	}
	s.resourceType = rt
	if err := handleTargetOSType(ctx, s); err != nil {
		return err
	}

	// Like upstream, never create the config file's folder.
	configFolder := filepath.Dir(s.configPath)
	if !isDir(configFolder) {
		return fmt.Errorf("Config file destination folder %s does not exist.", configFolder)
	}
	if s.credentialsFolder == "" {
		folder, err := defaultKeysFolderName(s, runtime.GOOS == "windows")
		if err != nil {
			return err
		}
		s.credentialsFolder = filepath.Join(configFolder, "az_ssh_config", folder)
	}

	_, err = doSSHOp(ctx, s, func(s *session, deleteKeys, deleteCert bool) (int, error) {
		return 0, writeSSHConfig(s, deleteKeys, deleteCert)
	})
	return err
}

func defaultKeysFolderName(s *session, windows bool) (string, error) {
	// * is not a valid folder name on Windows.
	name := s.ip
	if name == "*" {
		name = "all_ips"
	}
	if s.resourceGroup != "" && s.vmName != "" {
		name = s.resourceGroup + "-" + s.vmName
	}
	if windows && strings.ContainsAny(name, windowsInvalidFolderChars) && (s.localUser == "" || s.isArc()) {
		name = strings.Map(func(r rune) rune {
			if strings.ContainsRune(windowsInvalidFolderChars, r) {
				return -1
			}
			return r
		}, name)
		if name == "" {
			return "", errors.New("Can't create default folder for generated keys. Please provide --keys-destination-folder.")
		}
	}
	return name, nil
}

// writeSSHConfig is ssh_utils.write_ssh_config. deleteCert means AAD login.
func writeSSHConfig(s *session, deleteKeys, deleteCert bool) error {
	lines, err := configText(s, deleteCert)
	if err != nil {
		return err
	}
	issueConfigCleanupWarning(s, deleteCert, deleteKeys)

	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if s.overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(s.configPath, flags, 0o666)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(strings.Join(lines, "\n")); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// configText is ConfigSession.get_config_text. The leading "" gives the
// text a leading newline; there is no trailing one.
func configText(s *session, isAAD bool) ([]string, error) {
	lines := []string{""}
	if s.isArc() {
		path, err := createRelayInfoFile(s)
		if err != nil {
			return nil, err
		}
		s.relayInfoPath = path
		return append(lines, arcEntry(s, isAAD)...), nil
	}
	if s.resourceGroup != "" && s.vmName != "" && s.ip != "" {
		lines = append(lines, rgAndVMEntry(s, isAAD)...)
	}
	if s.ip == "" {
		s.ip = "*"
	}
	return append(lines, ipEntry(s, isAAD)...), nil
}

func credentialLines(s *session) []string {
	var lines []string
	if s.certFile != "" {
		lines = append(lines, "\tCertificateFile \""+s.certFile+"\"")
	}
	if s.privateKeyFile != "" {
		lines = append(lines, "\tIdentityFile \""+s.privateKeyFile+"\"")
	}
	return lines
}

func arcEntry(s *session, isAAD bool) []string {
	host := "Host " + s.resourceGroup + "-" + s.vmName
	if !isAAD {
		host += "-" + s.localUser
	}
	lines := []string{host, "\tHostName " + s.vmName, "\tUser " + s.localUser}
	lines = append(lines, credentialLines(s)...)
	proxy := "\tProxyCommand \"" + s.proxyPath + "\" -r \"" + s.relayInfoPath + "\""
	if s.port != "" {
		proxy += " -p " + s.port
	}
	return append(lines, proxy)
}

func rgAndVMEntry(s *session, isAAD bool) []string {
	host := "Host " + s.resourceGroup + "-" + s.vmName
	if !isAAD {
		host += "-" + s.localUser
	}
	lines := []string{host, "\tUser " + s.localUser, "\tHostName " + s.ip}
	lines = append(lines, credentialLines(s)...)
	if s.port != "" {
		lines = append(lines, "\tPort "+s.port)
	}
	return lines
}

func ipEntry(s *session, isAAD bool) []string {
	var lines []string
	if isAAD {
		lines = []string{"Host " + s.ip}
	} else {
		lines = []string{"Host " + s.ip + "-" + s.localUser, "\tHostName " + s.ip}
	}
	lines = append(lines, "\tUser "+s.localUser)
	lines = append(lines, credentialLines(s)...)
	if s.port != "" {
		lines = append(lines, "\tPort "+s.port)
	}
	return lines
}

// issueConfigCleanupWarning is _issue_config_cleanup_warning.
func issueConfigCleanupWarning(s *session, deleteCert, deleteKeys bool) {
	if deleteCert {
		if _, end, err := certificateTimes(s.certFile); err != nil {
			warnf("Couldn't determine certificate expiration. Error: %s", err)
		} else {
			successf("Generated SSH certificate %s is valid until %s in local time.", s.certFile, end.Local().Format("2006-01-02 03:04:05 PM"))
		}
	}
	if !deleteKeys && !deleteCert && !s.isArc() {
		return
	}

	var pathToDelete, items string
	switch {
	case s.isArc() && deleteKeys && deleteCert:
		pathToDelete = filepath.Dir(s.certFile) + " contains"
		items = " (id_rsa, id_rsa.pub, id_rsa.pub-aadcert.pub, " + filepath.Base(s.relayInfoPath) + ")"
	case s.isArc() && deleteCert:
		pathToDelete = s.certFile + " and " + s.relayInfoPath + " contain"
	case s.isArc():
		pathToDelete = s.relayInfoPath + " contains"
	case deleteKeys:
		pathToDelete = filepath.Dir(s.certFile) + " contains"
		items = " (id_rsa, id_rsa.pub, id_rsa.pub-aadcert.pub)"
	default:
		pathToDelete = s.certFile + " contains"
	}
	warnf("%s sensitive information%s. Please delete it once you no longer need this config file.", pathToDelete, items)
}

// createRelayInfoFile is ConfigSession._create_relay_info_file. The file
// holds a relay SAS key, so it is 0600 rather than upstream's 0644.
func createRelayInfoFile(s *session) (string, error) {
	if !isDir(s.credentialsFolder) {
		if err := os.MkdirAll(s.credentialsFolder, 0o755); err != nil {
			return "", err
		}
	}
	path := filepath.Join(s.credentialsFolder, s.resourceGroup+"-"+s.vmName+"-relay_info")
	if isFile(path) {
		if err := os.Remove(path); err != nil {
			return "", fmt.Errorf("%s already exists, and couldn't be overwritten.Error: %s", path, err)
		}
	}
	relay, err := s.relay.encode()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(relay), 0o600); err != nil {
		return "", fmt.Errorf("Couldn't write relay information to file %s.Error: %s", path, err)
	}
	expiry := time.Unix(s.relay.ExpiresOn, 0).Local().Format("2006-01-02 03:04:05 PM")
	successf("Generated relay information %s is valid until %s in local time.", path, expiry)
	return path, nil
}
