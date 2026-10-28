package ssh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// opFunc runs the final step (ssh, rdp, or write the config) and returns
// ssh's exit code.
type opFunc func(s *session, deleteKeys, deleteCert bool) (int, error)

// runVM is custom.ssh_vm; az ssh arc calls it with the placeholder type.
func runVM(ctx context.Context, s *session, debug bool) (int, error) {
	// Only the Azure Portal uses this, from Cloud Shell.
	if s.deleteCredentials && os.Getenv("AZUREPS_HOST_ENVIRONMENT") != "cloud-shell/1.0" {
		return 0, errors.New("Can't use --delete-private-key outside an Azure Cloud Shell session.")
	}
	if debug && !hasVerboseFlag(s.sshArgs) {
		s.sshArgs = append([]string{"-vvv"}, s.sshArgs...)
	}
	if err := assertArgs(s.resourceGroup, s.vmName, s.ip, s.resourceType, s.certFile, s.localUser); err != nil {
		return 0, err
	}

	op := opFunc(func(s *session, deleteKeys, deleteCert bool) (int, error) {
		return startSSHConnection(ctx, s, deleteKeys, deleteCert)
	})
	if s.winrdp {
		if runtime.GOOS != "windows" {
			return 0, errors.New("RDP connection is not supported for this platform. Supported platforms: Windows")
		}
		op = func(s *session, deleteKeys, deleteCert bool) (int, error) {
			return 0, startRDPConnection(ctx, s, deleteKeys, deleteCert)
		}
	}

	s.makePathsAbsolute()
	rt, err := decideResourceType(ctx, s)
	if err != nil {
		return 0, err
	}
	s.resourceType = rt
	if err := handleTargetOSType(ctx, s); err != nil {
		return 0, err
	}
	return doSSHOp(ctx, s, op)
}

// doSSHOp is custom._do_ssh_op.
func doSSHOp(ctx context.Context, s *session, op opFunc) (int, error) {
	// Resolve the IP first so a missing VM fails before keys exist.
	if !s.isArc() {
		if s.sshProxyFolder != "" {
			warnf("Target machine is not an Arc Server, --ssh-proxy-folder value will be ignored.")
		}
		if s.ip == "" {
			ip, err := getSSHIP(ctx, s)
			if err != nil {
				return 0, err
			}
			s.ip = ip
		}
		if s.ip == "" {
			if !s.usePrivateIP {
				return 0, fmt.Errorf("VM '%s' does not have a public IP address to SSH to", s.vmName)
			}
			return 0, errors.New("Internal Error. Couldn't determine the IP address.")
		}
	}

	// With --local-user the user's own credentials are used and kept.
	deleteKeys, deleteCert := false, false
	var certLifetime float64
	if s.localUser == "" {
		deleteCert = true
		pub, priv, generated, err := checkOrCreatePublicPrivateFiles(s.publicKeyFile, s.privateKeyFile, s.credentialsFolder, s.sshClientFolder)
		if err != nil {
			return 0, err
		}
		s.publicKeyFile, s.privateKeyFile, deleteKeys = pub, priv, generated
		certFile, user, err := getAndWriteCertificate(ctx, s.publicKeyFile, "")
		if err != nil {
			// Upstream leaks the temp keys here.
			if deleteKeys && s.credentialsFolder == "" {
				os.RemoveAll(filepath.Dir(s.privateKeyFile))
			}
			return 0, err
		}
		s.certFile, s.localUser = certFile, user
		if s.isArc() {
			if start, end, err := certificateTimes(s.certFile); err != nil {
				warnf("Couldn't determine certificate expiration. Error: %s", err)
			} else {
				certLifetime = end.Sub(start).Seconds()
			}
		}
	}

	if s.isArc() {
		if err := arcSetup(ctx, s, certLifetime); err != nil {
			if deleteKeys || deleteCert {
				newCleaner(s, deleteKeys, deleteCert).run()
			}
			return 0, err
		}
	}
	// Ctrl+C during the work above must not still start ssh.
	if err := ctx.Err(); err != nil {
		if deleteKeys || deleteCert {
			newCleaner(s, deleteKeys, deleteCert).run()
		}
		return 0, err
	}
	return op(s, deleteKeys, deleteCert)
}

func arcSetup(ctx context.Context, s *session, certLifetime float64) error {
	proxyPath, err := installClientSideProxy(ctx, s.sshProxyFolder)
	if err != nil {
		return err
	}
	s.proxyPath = proxyPath
	return getRelayInformation(ctx, s, certLifetime)
}

// runCert is custom.ssh_cert.
func runCert(ctx context.Context, certPath, pub, clientFolder string) error {
	if certPath == "" && pub == "" {
		return errors.New("--file or --public-key-file must be provided.")
	}
	// Absolute first: upstream rejects a bare "-f cert.pub" by accident.
	pub, certPath, clientFolder = absPath(pub), absPath(certPath), absPath(clientFolder)
	if certPath != "" && !isDir(filepath.Dir(certPath)) {
		return fmt.Errorf("%s folder doesn't exist", filepath.Dir(certPath))
	}

	// Without a public key, generate a pair next to --file.
	keysFolder := ""
	if pub == "" {
		keysFolder = filepath.Dir(certPath)
	}
	pub, _, _, err := checkOrCreatePublicPrivateFiles(pub, "", keysFolder, clientFolder)
	if err != nil {
		return err
	}
	certFile, _, err := getAndWriteCertificate(ctx, pub, certPath)
	if err != nil {
		return err
	}
	if keysFolder != "" {
		warnf("%s contains sensitive information (id_rsa, id_rsa.pub). Please delete once this certificate is no longer being used.", keysFolder)
	}
	if _, end, err := certificateTimes(certFile); err != nil {
		warnf("Couldn't determine certificate validity. Error: %s", err)
		successf("Generated SSH certificate %s.", certFile)
	} else {
		successf("Generated SSH certificate %s is valid until %s in local time.", certFile, end.Local().Format("2006-01-02 15:04:05"))
	}
	return nil
}
