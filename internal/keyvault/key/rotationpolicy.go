package key

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// rawPolicy mirrors the shape update_key_rotation_policy reads out of the
// parsed JSON (custom.py:1650). azure-cli runs the document through
// get_json_object, which snake-cases every key, so both timeAfterCreate and
// time_after_create reach that code as time_after_create. normalizeKeys does
// the same job here by folding every key to lowercase without underscores,
// which is why the json tags below have neither.
type rawPolicy struct {
	LifetimeActions []rawAction       `json:"lifetimeactions"`
	ExpiresIn       string            `json:"expiresin"`
	ExpiryTime      string            `json:"expirytime"`
	Attributes      map[string]string `json:"attributes"`
}

// rawAction accepts both the nested form ({"action": {"type": "rotate"},
// "trigger": {"timeAfterCreate": "P60D"}}) and the flattened form
// ({"action": "rotate", "timeAfterCreate": "P60D"}), as the Python does.
type rawAction struct {
	Action           json.RawMessage   `json:"action"`
	TimeAfterCreate  string            `json:"timeaftercreate"`
	TimeBeforeExpiry string            `json:"timebeforeexpiry"`
	Trigger          map[string]string `json:"trigger"`
}

// normalizeKeys lowercases every object key and drops underscores, so that
// lifetimeActions and lifetime_actions both match the tags on rawPolicy.
func normalizeKeys(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[strings.ReplaceAll(strings.ToLower(k), "_", "")] = normalizeKeys(val)
		}
		return out
	case []any:
		for i, val := range t {
			t[i] = normalizeKeys(val)
		}
		return t
	default:
		return v
	}
}

// parseRotationPolicy turns the --value argument into an SDK policy. The value
// is either a path to a JSON file or the JSON document itself; azure-cli tests
// the path first (os.path.exists), so an existing file wins.
func parseRotationPolicy(value string) (azkeys.KeyRotationPolicy, error) {
	var policy azkeys.KeyRotationPolicy

	data := []byte(value)
	if _, err := os.Stat(value); err == nil {
		data, err = os.ReadFile(value)
		if err != nil {
			return policy, fmt.Errorf("failed to read policy file: %w", err)
		}
	}

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return policy, errors.New("Please specify a valid policy")
	}
	normalized, err := json.Marshal(normalizeKeys(doc))
	if err != nil {
		return policy, errors.New("Please specify a valid policy")
	}
	var raw rawPolicy
	if err := json.Unmarshal(normalized, &raw); err != nil {
		return policy, errors.New("Please specify a valid policy")
	}

	actions := []*azkeys.LifetimeAction{}
	for _, a := range raw.LifetimeActions {
		action := &azkeys.LifetimeAction{}
		if t := actionType(a.Action); t != "" {
			action.Action = &azkeys.LifetimeActionType{Type: to.Ptr(azkeys.KeyRotationPolicyAction(t))}
		}
		after, before := a.TimeAfterCreate, a.TimeBeforeExpiry
		if len(a.Trigger) > 0 {
			after, before = a.Trigger["timeaftercreate"], a.Trigger["timebeforeexpiry"]
		}
		trigger := &azkeys.LifetimeActionTrigger{}
		if after != "" {
			trigger.TimeAfterCreate = to.Ptr(after)
		}
		if before != "" {
			trigger.TimeBeforeExpiry = to.Ptr(before)
		}
		action.Trigger = trigger
		actions = append(actions, action)
	}
	policy.LifetimeActions = actions

	// expiresIn wins over expiryTime, and a non-empty attributes object
	// shadows the top level entirely, even when it carries neither field.
	// An empty attributes object is falsy in Python, so it shadows nothing.
	expiresIn := firstNonEmpty(raw.ExpiresIn, raw.ExpiryTime)
	if len(raw.Attributes) > 0 {
		expiresIn = firstNonEmpty(raw.Attributes["expiresin"], raw.Attributes["expirytime"])
	}
	if expiresIn != "" {
		policy.Attributes = &azkeys.KeyRotationPolicyAttributes{ExpiryTime: to.Ptr(expiresIn)}
	}
	return policy, nil
}

// actionType reads the action name from either {"type": "rotate"} or "rotate".
func actionType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Type
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func ShowRotationPolicy(ctx context.Context, cmd *cobra.Command, vaultName, name string) error {
	client, err := rotationPolicyClient(vaultName)
	if err != nil {
		return err
	}
	resp, err := client.GetKeyRotationPolicy(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to get key rotation policy: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyRotationPolicy)
}

func UpdateRotationPolicy(ctx context.Context, cmd *cobra.Command, vaultName, name, value string) error {
	policy, err := parseRotationPolicy(value)
	if err != nil {
		return err
	}
	client, err := rotationPolicyClient(vaultName)
	if err != nil {
		return err
	}
	resp, err := client.UpdateKeyRotationPolicy(ctx, name, policy, nil)
	if err != nil {
		return fmt.Errorf("failed to update key rotation policy: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyRotationPolicy)
}

func rotationPolicyClient(vaultName string) (*azkeys.Client, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, err
	}
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create key client: %w", err)
	}
	return client, nil
}
