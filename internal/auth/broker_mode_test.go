package auth

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

func TestResolveBrokerMode(t *testing.T) {
	cases := []struct {
		flag, env, want string
		wantErr         bool
	}{
		{"", "", BrokerAuto, false},
		{"always", "", BrokerAlways, false},
		{"never", "", BrokerNever, false},
		{"auto", "false", BrokerAuto, false}, // flag beats env
		{"always", "false", BrokerAlways, false},
		{"", "false", BrokerNever, false},
		{"", " FALSE ", BrokerNever, false},
		{"", "0", BrokerNever, false},
		{"", "off", BrokerNever, false},
		{"", "no", BrokerNever, false},
		{"", "true", BrokerAuto, false},
		{"", "on", BrokerAuto, false},
		{"", "maybe", "", true},
		{"sometimes", "", "", true},
	}
	for _, tc := range cases {
		got, err := resolveBrokerMode(tc.flag, tc.env)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("resolveBrokerMode(%q, %q) = %q, %v; want %q, err=%v", tc.flag, tc.env, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestChooseAuthMode(t *testing.T) {
	ok := func() error { return nil }
	unavailable := func() error { return azure.ErrBrokerUnavailable }
	errStarted := errors.New("broker must not be started")
	mustNotStart := func() error { return errStarted }

	cases := []struct {
		name        string
		useAzureCLI bool
		mode        string
		start       func() error
		want        string
		errContains string
	}{
		{"auto uses broker", false, BrokerAuto, ok, azure.AuthModeBroker, ""},
		{"auto falls back silently", false, BrokerAuto, unavailable, azure.AuthModeDefault, ""},
		{"always uses broker", false, BrokerAlways, ok, azure.AuthModeBroker, ""},
		{"always fails without broker", false, BrokerAlways, unavailable, "", "--broker always"},
		{"never skips broker", false, BrokerNever, mustNotStart, azure.AuthModeDefault, ""},
		{"azure-cli skips broker", true, BrokerAuto, mustNotStart, azure.AuthModeAzureCLI, ""},
		{"azure-cli with never", true, BrokerNever, mustNotStart, azure.AuthModeAzureCLI, ""},
		{"azure-cli with always conflicts", true, BrokerAlways, mustNotStart, "", "--use-azure-cli"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := chooseAuthMode(tc.useAzureCLI, tc.mode, tc.start)
			if errors.Is(err, errStarted) {
				t.Fatal(err)
			}
			if tc.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("err = %v; want it to contain %q", err, tc.errContains)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestChooseAuthModeAlwaysKeepsCause(t *testing.T) {
	_, err := chooseAuthMode(false, BrokerAlways, func() error { return azure.ErrBrokerUnavailable })
	if runtime.GOOS == "windows" && !errors.Is(err, azure.ErrBrokerUnavailable) {
		t.Fatalf("err = %v; want it to wrap ErrBrokerUnavailable", err)
	}
	if runtime.GOOS != "windows" && !strings.Contains(err.Error(), "only available on Windows") {
		t.Fatalf("err = %v; want the Windows-only message", err)
	}
}
