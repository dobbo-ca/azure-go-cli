package ssh

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestAssertArgs(t *testing.T) {
	certFile := filepath.Join(t.TempDir(), "cert.pub")
	if err := os.WriteFile(certFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name                          string
		rg, vm, ip, rtype, cert, user string
		wantErr                       string
	}{
		{name: "ip only", ip: "10.0.0.4"},
		{name: "rg and vm", rg: "rg", vm: "vm"},
		{name: "legacy type", rg: "rg", vm: "vm", rtype: "Microsoft.Compute"},
		{name: "arc placeholder", rg: "rg", vm: "vm", rtype: arcResourceTypePlaceholder},
		{name: "bad type", ip: "1.2.3.4", rtype: "Microsoft.Web/sites", wantErr: "--resource-type must be either"},
		{name: "nothing", wantErr: "The VM must be specified by --ip or --resource-group and --vm-name/--name"},
		{name: "rg alone", rg: "rg", wantErr: "--resource-group and --vm-name/--name must be provided together"},
		{name: "ip and vm", ip: "1.2.3.4", rg: "rg", vm: "vm", wantErr: "--ip cannot be used with --resource-group or --vm-name/--name"},
		{name: "cert without user", ip: "1.2.3.4", cert: certFile, wantErr: "To authenticate with a certificate you need to provide a --local-user"},
		{name: "missing cert", ip: "1.2.3.4", cert: "/nope/cert.pub", user: "u", wantErr: "Certificate file /nope/cert.pub not found"},
		{name: "cert with user", ip: "1.2.3.4", cert: certFile, user: "u"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := assertArgs(tt.rg, tt.vm, tt.ip, tt.rtype, tt.cert, tt.user)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestPickIP(t *testing.T) {
	both := []ipConfig{{private: "10.0.0.4"}, {private: "10.0.0.5", public: "20.1.2.3"}}
	privOnly := []ipConfig{{private: "10.0.0.4"}, {private: "10.0.0.5"}}
	tests := []struct {
		name         string
		cfgs         []ipConfig
		usePrivate   bool
		wantIP       string
		wantFallback bool
	}{
		{"public wins", both, false, "20.1.2.3", false},
		{"prefer private", both, true, "10.0.0.4", false},
		{"fallback to first private", privOnly, false, "10.0.0.4", true},
		{"prefer private, public only", []ipConfig{{public: "20.1.2.3"}}, true, "20.1.2.3", false},
		{"no ips", nil, false, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, fallback := pickIP(tt.cfgs, tt.usePrivate)
			if ip != tt.wantIP || fallback != tt.wantFallback {
				t.Errorf("pickIP() = %q, %v; want %q, %v", ip, fallback, tt.wantIP, tt.wantFallback)
			}
		})
	}
}

func TestPrepareJWKDataMatchesPython(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubKey, err := gossh.NewPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubFile := filepath.Join(t.TempDir(), "id_rsa.pub")
	// ssh-keygen output: "<alg> <base64> <comment>\n".
	line := strings.TrimSuffix(string(gossh.MarshalAuthorizedKey(pubKey)), "\n") + " user@host\n"
	if err := os.WriteFile(pubFile, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	req, err := prepareJWKData(pubFile)
	if err != nil {
		t.Fatal(err)
	}
	// Python keeps the mpint's leading zero byte and the base64 padding.
	n := base64.URLEncoding.EncodeToString(append([]byte{0}, key.N.Bytes()...))
	e := base64.URLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
	sum := sha256.Sum256([]byte(n + e))
	kid := hex.EncodeToString(sum[:])
	want := `{"kty": "RSA", "n": "` + n + `", "e": "` + e + `", "kid": "` + kid + `"}`
	if req.ReqCnf != want || req.KeyID != kid || req.TokenType != "ssh-cert" {
		t.Errorf("prepareJWKData() = %+v\nwant req_cnf %s", req, want)
	}
	if e != "AQAB" {
		t.Errorf("exponent = %s, want AQAB", e)
	}
}

func TestParseRSAPublicKeyErrors(t *testing.T) {
	tests := map[string]string{
		"nospace":                   "Incorrectly formatted public key",
		"ssh-ed25519 AAAA":          "Public key is not ssh-rsa algorithm (ssh-ed25519)",
		"ssh-rsa " + b64Fields("a"): "Incorrectly encoded public key",
		"ssh-rsa " + b64Fields("ssh-dss", "e", "n"): "Encoded public key is not ssh-rsa algorithm (ssh-dss)",
	}
	for text, want := range tests {
		if _, _, err := parseRSAPublicKey(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseRSAPublicKey(%q) error = %v, want %q", text, err, want)
		}
	}
}

func b64Fields(fields ...string) string {
	var b []byte
	for _, f := range fields {
		b = append(b, byte(len(f)>>24), byte(len(f)>>16), byte(len(f)>>8), byte(len(f)))
		b = append(b, f...)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func TestBuildArgs(t *testing.T) {
	s := &session{ip: "10.0.0.4", resourceType: typeCompute, privateKeyFile: `C:\Users\A B\id_rsa`,
		certFile: `C:\Users\A B\id_rsa.pub-aadcert.pub`, port: "2222"}
	want := []string{"-i", `C:\Users\A B\id_rsa`, "-o", `CertificateFile="C:\Users\A B\id_rsa.pub-aadcert.pub"`, "-p", "2222"}
	if got := s.buildArgs(); !reflect.DeepEqual(got, want) {
		t.Errorf("buildArgs() = %q, want %q", got, want)
	}

	arc := &session{vmName: "m", resourceType: typeHybridCompute, proxyPath: "/p/sshProxy", port: "2222"}
	want = []string{"-o", `ProxyCommand="/p/sshProxy" -p 2222`}
	if got := arc.buildArgs(); !reflect.DeepEqual(got, want) {
		t.Errorf("arc buildArgs() = %q, want %q", got, want)
	}
	if host, _ := arc.host(); host != "m" {
		t.Errorf("arc host() = %q, want m", host)
	}
}

func TestConfigText(t *testing.T) {
	aad := &session{resourceGroup: "myRG", vmName: "myVM", ip: "20.1.2.3", localUser: "user@contoso.com",
		certFile: "/k/id_rsa.pub-aadcert.pub", privateKeyFile: "/k/id_rsa", port: "2222", resourceType: typeCompute}
	got, err := configText(aad, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "\nHost myRG-myVM\n\tUser user@contoso.com\n\tHostName 20.1.2.3\n" +
		"\tCertificateFile \"/k/id_rsa.pub-aadcert.pub\"\n\tIdentityFile \"/k/id_rsa\"\n\tPort 2222\n" +
		"Host 20.1.2.3\n\tUser user@contoso.com\n" +
		"\tCertificateFile \"/k/id_rsa.pub-aadcert.pub\"\n\tIdentityFile \"/k/id_rsa\"\n\tPort 2222"
	if j := strings.Join(got, "\n"); j != want {
		t.Errorf("AAD config =\n%q\nwant\n%q", j, want)
	}

	local := &session{ip: "10.0.0.4", localUser: "azureuser", privateKeyFile: "/k/key", resourceType: typeCompute}
	got, _ = configText(local, false)
	want = "\nHost 10.0.0.4-azureuser\n\tHostName 10.0.0.4\n\tUser azureuser\n\tIdentityFile \"/k/key\""
	if j := strings.Join(got, "\n"); j != want {
		t.Errorf("local user config =\n%q\nwant\n%q", j, want)
	}

	all := &session{localUser: "u", resourceType: typeCompute}
	got, _ = configText(all, true)
	if j := strings.Join(got, "\n"); j != "\nHost *\n\tUser u" {
		t.Errorf("wildcard config = %q", j)
	}
}

func TestArcEntry(t *testing.T) {
	s := &session{resourceGroup: "rg", vmName: "m", localUser: "u", certFile: "/k/c", privateKeyFile: "/k/id_rsa",
		proxyPath: "/p/sshProxy", relayInfoPath: "/k/rg-m-relay_info", port: "2222"}
	want := []string{"Host rg-m", "\tHostName m", "\tUser u", "\tCertificateFile \"/k/c\"", "\tIdentityFile \"/k/id_rsa\"",
		"\tProxyCommand \"/p/sshProxy\" -r \"/k/rg-m-relay_info\" -p 2222"}
	if got := arcEntry(s, true); !reflect.DeepEqual(got, want) {
		t.Errorf("arcEntry() = %q, want %q", got, want)
	}
}

func TestDefaultKeysFolderName(t *testing.T) {
	tests := []struct {
		s       session
		windows bool
		want    string
		wantErr bool
	}{
		{session{ip: "10.0.0.4"}, false, "10.0.0.4", false},
		{session{ip: "*"}, true, "all_ips", false},
		{session{resourceGroup: "rg", vmName: "vm"}, false, "rg-vm", false},
		{session{ip: "host:22"}, true, "host22", false},
		{session{ip: "host:22", localUser: "u", resourceType: typeCompute}, true, "host:22", false},
		{session{ip: "*?"}, true, "", true},
	}
	for _, tt := range tests {
		got, err := defaultKeysFolderName(&tt.s, tt.windows)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("defaultKeysFolderName(%+v, %v) = %q, %v; want %q", tt.s, tt.windows, got, err, tt.want)
		}
	}
}

func TestRelayInfoEncode(t *testing.T) {
	r := &relayInfo{NamespaceName: "ns", NamespaceNameSuffix: "servicebus.windows.net", HybridConnectionName: "hc",
		AccessKey: "SharedAccessSignature sr=x&sig=y", ExpiresOn: 1620000256, ServiceConfigurationToken: "tok"}
	got, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(got)
	want := `{"relay": {"namespaceName": "ns", "namespaceNameSuffix": "servicebus.windows.net", "hybridConnectionName": "hc", ` +
		`"accessKey": "SharedAccessSignature sr=x&sig=y", "expiresOn": 1620000256, "serviceConfigurationToken": "tok"}}`
	if string(raw) != want {
		t.Errorf("encode() decodes to\n%s\nwant\n%s", raw, want)
	}
}

func TestProxyTarFiles(t *testing.T) {
	build := func(names ...string) []byte {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: "ssh-proxy_linux_amd64/", Typeflag: tar.TypeDir, Mode: 0o755})
		for _, n := range names {
			tw.WriteHeader(&tar.Header{Name: n, Typeflag: tar.TypeReg, Mode: 0o755, Size: 1})
			tw.Write([]byte("x"))
		}
		tw.Close()
		gz.Close()
		return buf.Bytes()
	}
	files, err := proxyTarFiles(build("d/sshproxy", "d/LICENSE.txt", "d/ThirdPartyNotice.txt"), "sshProxy_linux_amd64_1_3_033291")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"sshProxy_linux_amd64_1_3_033291", "LICENSE.txt", "ThirdPartyNotice.txt"} {
		if _, ok := files[n]; !ok {
			t.Errorf("missing %s in %v", n, files)
		}
	}
	if _, err := proxyTarFiles(build("d/evil.sh"), "p"); err == nil {
		t.Error("unexpected file accepted")
	}
	if _, err := proxyTarFiles(build("d/sub/sshproxy"), "p"); err == nil {
		t.Error("nested path accepted")
	}
}

func TestAgentTooOld(t *testing.T) {
	tests := map[string]bool{"1.30.0.0": true, "0.99.1.2": true, "1.31.0.0": false, "2.0.0.0": false, "1.30": false, "x.y.z.w": false}
	for v, want := range tests {
		if got := agentTooOld(v); got != want {
			t.Errorf("agentTooOld(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestOpenSSHVersion(t *testing.T) {
	major, minor, ok := openSSHVersion("OpenSSH_for_Windows_8.1p1, LibreSSL 3.0.2")
	if !ok || major != 8 || minor != 1 {
		t.Errorf("openSSHVersion() = %d.%d %v, want 8.1", major, minor, ok)
	}
	major, minor, ok = openSSHVersion("debug1: Remote protocol version 2.0, remote software version OpenSSH_7.4")
	if !ok || major != 7 || minor != 4 {
		t.Errorf("openSSHVersion() = %d.%d %v, want 7.4", major, minor, ok)
	}
}

func TestCleanerRemovesGeneratedCredentials(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "aadsshcert123")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := &session{privateKeyFile: filepath.Join(dir, "id_rsa"), publicKeyFile: filepath.Join(dir, "id_rsa.pub"),
		certFile: filepath.Join(dir, "id_rsa.pub-aadcert.pub")}
	for _, p := range []string{s.privateKeyFile, s.publicKeyFile, s.certFile} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c := newCleaner(s, true, true)
	c.run()
	c.run()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("temp folder still exists: %v", err)
	}
}
