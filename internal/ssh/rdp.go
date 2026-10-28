package ssh

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// tunnel is an ssh -L process whose stderr we read line by line. stderr is
// an os.Pipe, not StderrPipe, so Wait can run while we are still reading.
type tunnel struct {
	cmd    *exec.Cmd
	stderr *bufio.Reader
	pipe   *os.File
	done   chan struct{}
	// tail gets the rest of stderr once a background drain is started.
	tail chan string
}

func (t *tunnel) exited() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

func (t *tunnel) exitCode() int {
	if !t.exited() {
		return -1
	}
	return t.cmd.ProcessState.ExitCode()
}

// startRDPConnection is rdp_utils.start_rdp_connection. Like upstream, a
// failed tunnel prints ssh's errors but is not itself an error.
func startRDPConnection(ctx context.Context, s *session, deleteKeys, deleteCert bool) error {
	c := newCleaner(s, deleteKeys, deleteCert)
	var t *tunnel
	var logs []string
	printLogs := false
	defer func() {
		terminateSSH(t, logs, printLogs)
		c.run()
		if deleteKeys {
			deleteFolderWarn(filepath.Dir(s.certFile))
		}
	}()

	localPort, err := openLocalPort()
	if err != nil {
		return err
	}
	s.sshArgs = append([]string{"-L", fmt.Sprintf("%d:localhost:3389", localPort), "-N"}, s.sshArgs...)

	success := false
	for attempt, allowed := 0, 0; attempt <= allowed && !success; attempt++ {
		if attempt == 1 {
			t.pipe.Close()
			warnf("SSH connection failed, possibly caused by new service configuration setup. Retrying the connection in %d seconds.", int(retryDelay.Seconds()))
			if err := sleepCtx(ctx, retryDelay); err != nil {
				return err
			}
		}
		t, printLogs, err = startSSHTunnel(s)
		if err != nil {
			return err
		}
		var delayError bool
		success, logs, delayError = waitForSSHConnection(t, printLogs)
		if s.newServiceConfig && delayError && t.exitCode() == 255 {
			allowed = 1
			if attempt == 1 {
				warnf("SSH connection failure could still be due to Service Configuration update. Please re-run command.")
			}
		}
	}

	c.run()
	if err := ctx.Err(); err != nil {
		return err
	}
	if success && !t.exited() {
		// Keep reading so ssh -v never blocks on a full pipe.
		t.drain(printLogs)
		return callRDP(ctx, localPort)
	}
	return nil
}

// openLocalPort is _get_open_port plus the is_local_port_open loop.
func openLocalPort() (int, error) {
	for {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 200*time.Millisecond)
		if err != nil {
			logger.Info("Port %d is open", port)
			return port, nil
		}
		conn.Close()
		logger.Info("Port %d is NOT open", port)
	}
}

// startSSHTunnel is rdp_utils.start_ssh_tunnel, without upstream's
// shell=True wrapper around ssh.
func startSSHTunnel(s *session) (*tunnel, bool, error) {
	env := os.Environ()
	if s.isArc() {
		relay, err := s.relay.encode()
		if err != nil {
			return nil, false, err
		}
		env = append(env, "SSHPROXY_RELAY_INFO="+relay)
	}
	printLogs := hasVerboseFlag(s.sshArgs)
	for _, a := range s.sshArgs {
		if a == "-E" {
			return nil, false, errors.New("Can't use -E ssh parameter when using --rdp")
		}
	}

	sshPath, err := getSSHClientPath("ssh", s.sshClientFolder)
	if err != nil {
		return nil, false, err
	}
	host, err := s.host()
	if err != nil {
		return nil, false, err
	}
	command := []string{host, "-l", s.localUser}
	if !printLogs {
		command = append(command, "-v")
	}
	command = append(command, s.buildArgs()...)
	command = append(command, s.sshArgs...)
	logger.Debug("Running ssh command %s %s", sshPath, strings.Join(command, " "))

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, false, err
	}
	cmd := exec.Command(sshPath, command...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, false, withRecommendation(fmt.Sprintf("Failed to run ssh command with error: %s.", err), recommendationSSHClientNotFound)
	}
	pw.Close()
	t := &tunnel{cmd: cmd, stderr: bufio.NewReader(pr), pipe: pr, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(t.done)
	}()
	return t, printLogs, nil
}

// waitForSSHConnection is rdp_utils.wait_for_ssh_connection.
func waitForSSHConnection(t *tunnel, printLogs bool) (bool, []string, bool) {
	var logs []string
	delayError := false
	for {
		line, err := t.stderr.ReadString('\n')
		if printLogs {
			fmt.Print(line)
		} else {
			logs = append(logs, line)
		}
		if strings.Contains(line, "debug1: Entering interactive session.") {
			logger.Debug("SSH Connection established succesfully.")
			return true, logs, delayError
		}
		if err != nil {
			<-t.done
		}
		if t.exited() {
			logger.Debug("SSH Connection failed.")
			return false, logs, delayError
		}
		if !delayError {
			delayError = serviceConfigDelayRe.MatchString(line)
		}
	}
}

// drain reads the rest of stderr in the background, printing it live when
// the user asked for ssh logs and collecting it otherwise.
func (t *tunnel) drain(printLogs bool) {
	t.tail = make(chan string, 1)
	go func() {
		var b strings.Builder
		for {
			line, err := t.stderr.ReadString('\n')
			if printLogs {
				fmt.Print(line)
			} else {
				b.WriteString(line)
			}
			if err != nil {
				t.tail <- b.String()
				return
			}
		}
	}()
}

// terminateSSH is rdp_utils.terminate_ssh plus print_error_messages_from_log.
func terminateSSH(t *tunnel, logs []string, printLogs bool) {
	if t == nil {
		return
	}
	terminated := false
	if !t.exited() {
		killProcessTree(t.cmd.Process)
		select {
		case <-t.done:
		case <-time.After(rdpTerminateWait):
		}
		terminated = true
	}
	if t.exited() && t.tail == nil {
		t.drain(printLogs)
	}
	if t.tail != nil {
		// A surviving grandchild can hold the pipe open; don't hang on it.
		select {
		case rest := <-t.tail:
			if rest != "" {
				logs = append(logs, strings.SplitAfter(rest, "\n")...)
			}
		case <-time.After(5 * time.Second):
		}
	}
	t.pipe.Close()
	if t.exitCode() != 0 && !printLogs && !terminated {
		for _, line := range logs {
			if line != "" && !strings.Contains(line, "debug1:") {
				fmt.Print(line)
			}
		}
	}
}

// killProcessTree also kills sshproxy, which ssh starts as a child.
func killProcessTree(p *os.Process) {
	err := exec.Command("taskkill", "/PID", strconv.Itoa(p.Pid), "/T", "/F").Run()
	if err != nil {
		if kerr := p.Kill(); kerr != nil {
			warnf("Kill process failed. Process no longer exists: %s", kerr)
		}
	}
}

// callRDP is rdp_utils.call_rdp. Ctrl+C ends the wait so the tunnel and
// credentials are cleaned up; the mstsc window is left open.
func callRDP(ctx context.Context, localPort int) error {
	rdpPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "mstsc.exe")
	logger.Debug("Attempting to run rdp from path %s", rdpPath)
	if !isFile(rdpPath) {
		return errors.New("Could not find mstsc.exe. Is the rdp client installed?")
	}
	successf("Launching Remote Desktop Connection")
	successf("To close this session, close the Remote Desktop Connection window.")
	cmd := exec.Command(rdpPath, "/v:localhost:"+strconv.Itoa(localPort))
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return nil
}
