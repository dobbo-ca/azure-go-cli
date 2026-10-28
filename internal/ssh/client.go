package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

var (
	openSSHVersionRe     = regexp.MustCompile(`OpenSSH.*_([0-9]+)\.([0-9]+)`)
	proxyCopyErrorRe     = regexp.MustCompile(regexp.QuoteMeta(`{"level":"fatal","msg":"sshproxy: error copying information from the connection: `) + `.*` + regexp.QuoteMeta(`","time":"`) + `.*` + regexp.QuoteMeta(`"}`))
	serviceConfigDelayRe = regexp.MustCompile(regexp.QuoteMeta(`{"level":"fatal","msg":"sshproxy: error connecting to the address: 404 Endpoint does not exist`))
)

// getSSHClientPath is ssh_utils.get_ssh_client_path. On Windows it only
// uses the OpenSSH that ships with the OS, never PATH.
func getSSHClientPath(command, clientFolder string) (string, error) {
	if clientFolder != "" {
		p := filepath.Join(clientFolder, command)
		if runtime.GOOS == "windows" {
			p += ".exe"
		}
		if isFile(p) {
			logger.Debug("Attempting to run %s from path %s", command, p)
			return p, nil
		}
		warnf("Could not find %s in provided --ssh-client-folder %s. Attempting to get pre-installed OpenSSH bits.", command, clientFolder)
	}
	if runtime.GOOS != "windows" {
		return command, nil
	}

	// A 32-bit process on a 64-bit OS sees SysWOW64 as System32.
	sysDir := "System32"
	if runtime.GOARCH == "386" && os.Getenv("PROCESSOR_ARCHITEW6432") != "" {
		sysDir = "SysNative"
	}
	p := filepath.Join(os.Getenv("SystemRoot"), sysDir, "OpenSSH", command+".exe")
	logger.Debug("Attempting to run %s from path %s", command, p)
	if !isFile(p) {
		return "", withRecommendation("Could not find "+command+".exe on path "+p+". ",
			"Make sure OpenSSH is installed correctly: https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse . "+
				"Or use --ssh-client-folder to provide folder path with ssh executables. ")
	}
	return p, nil
}

// cleaner is ssh_utils.do_cleanup. It is safe to run more than once and
// from more than one goroutine.
type cleaner struct {
	mu                                        sync.Mutex
	deleteKeys, deleteCert, deleteCredentials bool
	certFile, privateKey, publicKey           string
}

func newCleaner(s *session, deleteKeys, deleteCert bool) *cleaner {
	return &cleaner{deleteKeys: deleteKeys, deleteCert: deleteCert, deleteCredentials: s.deleteCredentials,
		certFile: s.certFile, privateKey: s.privateKeyFile, publicKey: s.publicKeyFile}
}

func (c *cleaner) run() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if (c.deleteKeys || c.deleteCredentials) && c.privateKey != "" {
		deleteFileWarn(c.privateKey, fmt.Sprintf("Couldn't delete private key %s. ", c.privateKey))
	}
	if c.deleteKeys && c.publicKey != "" {
		deleteFileWarn(c.publicKey, fmt.Sprintf("Couldn't delete public key %s. ", c.publicKey))
	}
	if (c.deleteCert || c.deleteCredentials) && c.certFile != "" {
		deleteFileWarn(c.certFile, fmt.Sprintf("Couldn't delete certificate %s. ", c.certFile))
	}
	if c.deleteKeys && c.certFile != "" {
		// Generated keys always live in their own temp folder.
		deleteFolderWarn(filepath.Dir(c.certFile))
	}
}

func deleteFileWarn(path, message string) {
	if isFile(path) {
		if err := os.Remove(path); err != nil {
			warnf("%s", message)
		}
	}
}

func deleteFolderWarn(dir string) {
	if isDir(dir) {
		if err := os.Remove(dir); err != nil {
			warnf("Couldn't delete temporary folder %s", dir)
		}
	}
}

// startSSHConnection is ssh_utils.start_ssh_connection. It returns ssh's
// exit code.
func startSSHConnection(ctx context.Context, s *session, deleteKeys, deleteCert bool) (int, error) {
	c := newCleaner(s, deleteKeys, deleteCert)
	// Credentials go even if something fails before ssh exits.
	defer c.run()

	args := s.sshArgs
	// Read ssh's logs to delete credentials once auth succeeds and to
	// explain well-known failures.
	redirect := !hasVerboseFlag(args) && (deleteCert || s.deleteCredentials)
	if redirect {
		args = append([]string{"-v"}, args...)
	}

	env := os.Environ()
	if s.isArc() {
		relay, err := s.relay.encode()
		if err != nil {
			return 0, err
		}
		env = append(env, "SSHPROXY_RELAY_INFO="+relay)
	}

	sshPath, err := getSSHClientPath("ssh", s.sshClientFolder)
	if err != nil {
		return 0, err
	}
	host, err := s.host()
	if err != nil {
		return 0, err
	}
	command := append([]string{host, "-l", s.localUser}, s.buildArgs()...)
	command = append(command, args...)
	logger.Debug("Running ssh command %s %s", sshPath, strings.Join(command, " "))

	exitCode := 0
	for attempt, allowed, ok := 0, 0, false; attempt <= allowed && !ok; attempt++ {
		if attempt == 1 {
			warnf("SSH connection failed, possibly caused by new service configuration setup. Retrying the connection in %d seconds.", int(retryDelay.Seconds()))
			if err := sleepCtx(ctx, retryDelay); err != nil {
				return exitCode, err
			}
		}
		cmd := exec.Command(sshPath, command...)
		cmd.Env = env
		cmd.Stdin, cmd.Stdout = os.Stdin, os.Stdout
		delayError := false
		if redirect {
			stderr, err := cmd.StderrPipe()
			if err != nil {
				return 0, err
			}
			if err := cmd.Start(); err != nil {
				return 0, withRecommendation(fmt.Sprintf("Failed to run ssh command with error: %s.", err), recommendationSSHClientNotFound)
			}
			delayError = checkSSHLogsForCommonErrors(stderr, c, deleteCert)
			_ = cmd.Wait()
		} else {
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				return 0, withRecommendation(fmt.Sprintf("Failed to run ssh command with error: %s.", err), recommendationSSHClientNotFound)
			}
			waitToDeleteCredentials(cmd, c, deleteCert || s.deleteCredentials)
		}
		exitCode = cmd.ProcessState.ExitCode()
		ok = exitCode == 0
		if s.newServiceConfig && (delayError || (exitCode == 255 && !redirect)) {
			allowed = 1
			if attempt == 1 {
				warnf("SSH connection failure could still be due to Service Configuration update. Please re-run command.")
			}
		}
	}
	return exitCode, nil
}

// checkSSHLogsForCommonErrors is _check_ssh_logs_for_common_errors. It
// echoes everything but debug lines and returns whether the Arc proxy
// reported a new service configuration that is not live yet.
func checkSSHLogsForCommonErrors(stderr io.Reader, c *cleaner, deleteCert bool) bool {
	// Upstream only checks the deadline when a line arrives.
	timer := time.AfterFunc(cleanupTimeLimit, c.run)
	defer timer.Stop()

	var logs []string
	delayError := false
	r := bufio.NewReader(stderr)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			logs = append(logs, line)
			if !strings.HasPrefix(line, "debug1:") && !strings.HasPrefix(line, "debug2:") &&
				!strings.HasPrefix(line, "debug3:") && !strings.HasPrefix(line, "Authenticated ") {
				os.Stderr.WriteString(line)
				checkForKnownErrors(line, deleteCert, logs)
				if !delayError {
					delayError = serviceConfigDelayRe.MatchString(line)
				}
			}
			if strings.Contains(line, "debug1: Entering interactive session.") {
				timer.Stop()
				c.run()
			}
		}
		if err != nil {
			return delayError
		}
	}
}

// waitToDeleteCredentials is _wait_to_delete_credentials.
func waitToDeleteCredentials(cmd *exec.Cmd, c *cleaner, deleteCreds bool) {
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	if deleteCreds {
		select {
		case <-done:
		case <-time.After(cleanupTimeLimit):
		}
		c.run()
	}
	<-done
}

// checkForKnownErrors is _check_for_known_errors.
func checkForKnownErrors(line string, deleteCert bool, logs []string) {
	if strings.Contains(line, "Permission denied (publickey).") && deleteCert {
		warnOpenSSHMismatch(logs)
	}
	if proxyCopyErrorRe.MatchString(line) {
		fmt.Fprintln(os.Stderr, "ERROR: Please make sure SSH port is allowed using \"azcmagent config list\" in the target Arc Server. Ensure SSHD is running on the target machine.")
	}
}

// warnOpenSSHMismatch explains the RSA SHA-1 break between OpenSSH 7.8
// servers and 8.8 clients, which shows up as "Permission denied".
func warnOpenSSHMismatch(logs []string) {
	if len(logs) == 0 {
		return
	}
	localMajor, localMinor, ok := openSSHVersion(logs[0])
	if !ok {
		return
	}
	var remoteMajor, remoteMinor int
	found := false
	for _, l := range logs {
		if strings.Contains(l, "remote software version") {
			remoteMajor, remoteMinor, found = openSSHVersion(l)
			break
		}
	}
	if !found {
		return
	}
	older := func(major, minor int) bool { return major < 7 || (major == 7 && minor < 8) }
	newer := func(major, minor int) bool { return major > 8 || (major == 8 && minor >= 8) }
	switch {
	case older(remoteMajor, remoteMinor) && newer(localMajor, localMinor):
		warnf("The OpenSSH server version in the target VM %d.%d is too old. Version incompatible with OpenSSH client version %d.%d. "+
			"Refer to \"Potentially-incompatible changes\" in https://www.openssh.org/txt/release-8.8 for more information.",
			remoteMajor, remoteMinor, localMajor, localMinor)
	case older(localMajor, localMinor) && newer(remoteMajor, remoteMinor):
		warnf("The OpenSSH client version %d.%d is too old. Version incompatible with OpenSSH server version %d.%d in the target VM. "+
			"Refer to \"Potentially-incompatible changes\" in https://www.openssh.org/txt/release-8.8 for more information.",
			localMajor, localMinor, remoteMajor, remoteMinor)
	}
}

func openSSHVersion(line string) (int, int, bool) {
	m := openSSHVersionRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	return major, minor, err1 == nil && err2 == nil
}
