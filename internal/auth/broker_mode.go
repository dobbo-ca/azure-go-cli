package auth

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

// Broker modes for az login --broker.
const (
	BrokerAuto   = "auto"   // WAM when available, browser retry if it fails
	BrokerAlways = "always" // WAM only; never fall back to the browser
	BrokerNever  = "never"  // browser only; msalruntime.dll is never loaded
)

// brokerEnvVar is Python azure-cli's environment form of
// `az config set core.enable_broker_on_windows=false`.
const brokerEnvVar = "AZURE_CORE_ENABLE_BROKER_ON_WINDOWS"

// resolveBrokerMode picks the broker mode from the --broker flag, then the
// AZURE_CORE_ENABLE_BROKER_ON_WINDOWS environment variable, then auto. The
// variable takes the boolean spellings Python's configparser accepts: false
// means never, true means auto (Python's default).
func resolveBrokerMode(flag, env string) (string, error) {
	switch flag {
	case BrokerAuto, BrokerAlways, BrokerNever:
		return flag, nil
	case "":
	default:
		return "", fmt.Errorf("invalid --broker value %q: use auto, always or never", flag)
	}
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "":
		return BrokerAuto, nil
	case "1", "yes", "true", "on":
		return BrokerAuto, nil
	case "0", "no", "false", "off":
		return BrokerNever, nil
	default:
		return "", fmt.Errorf("invalid %s value %q: use true or false", brokerEnvVar, env)
	}
}

// chooseAuthMode decides the sign-in mode for az login. startBroker loads the
// WAM broker; it is not called in azure-cli mode or with --broker never.
func chooseAuthMode(useAzureCLI bool, brokerMode string, startBroker func() error) (string, error) {
	if useAzureCLI {
		if brokerMode == BrokerAlways {
			return "", fmt.Errorf("--broker always cannot be combined with --use-azure-cli")
		}
		return azure.AuthModeAzureCLI, nil
	}
	if brokerMode == BrokerNever {
		return azure.AuthModeDefault, nil
	}
	if err := startBroker(); err != nil {
		if brokerMode == BrokerAlways {
			if runtime.GOOS != "windows" {
				return "", fmt.Errorf("--broker always: the WAM broker is only available on Windows")
			}
			return "", fmt.Errorf("--broker always: the WAM broker is unavailable: %w", err)
		}
		// A broker that won't start is normal, not an error worth showing.
		return azure.AuthModeDefault, nil
	}
	return azure.AuthModeBroker, nil
}
