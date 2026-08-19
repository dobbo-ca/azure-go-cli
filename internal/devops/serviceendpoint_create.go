package devops

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"unicode/utf16"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// serviceendpointFileEncodings mirrors FILE_ENCODING_TYPES
// (dev/common/utils.py:11), the --encoding choice list.
var serviceendpointFileEncodings = []string{"ascii", "utf-16be", "utf-16le", "utf-8"}

func serviceendpointNewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a service endpoint using configuration file.",
		Long:  "You can learn more about this at https://aka.ms/azure-devops-service-endpoint-config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("service-endpoint-configuration", "", "Configuration file with service endpoint request.")
	_ = cmd.MarkFlagRequired("service-endpoint-configuration")
	cmd.Flags().String("encoding", "utf-8", "Encoding of the input file.")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func serviceendpointRunCreate(ctx context.Context, cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("service-endpoint-configuration")
	encoding, _ := cmd.Flags().GetString("encoding")

	content, err := serviceendpointReadFile(path, encoding)
	if err != nil {
		return err
	}

	// service_endpoint.py:196-201: the parsed JSON is sent verbatim as the
	// request body, with no client-side field shaping.
	var config any
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var endpoint map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "serviceendpoint/endpoints",
		APIVersion: "5.0-preview.2",
		Body:       config,
	}, &endpoint); err != nil {
		return fmt.Errorf("failed to create service endpoint: %w", err)
	}

	// No table transformer (commands.py:114).
	return ado.Print(cmd, endpoint)
}

// serviceendpointReadFile reads path and decodes it to UTF-8 text per
// encoding, matching read_file_content (dev/common/utils.py:13-18): reject
// any encoding outside FILE_ENCODING_TYPES before touching the file.
func serviceendpointReadFile(path, encoding string) ([]byte, error) {
	valid := false
	for _, e := range serviceendpointFileEncodings {
		if e == encoding {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("File encoding %s is not supported.", encoding)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	switch encoding {
	case "utf-16be", "utf-16le":
		return serviceendpointDecodeUTF16(raw, encoding == "utf-16be")
	default: // ascii, utf-8: already valid UTF-8 input, no conversion needed
		return raw, nil
	}
}

func serviceendpointDecodeUTF16(raw []byte, bigEndian bool) ([]byte, error) {
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("invalid utf-16 file: odd number of bytes")
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		if bigEndian {
			units[i] = binary.BigEndian.Uint16(raw[i*2:])
		} else {
			units[i] = binary.LittleEndian.Uint16(raw[i*2:])
		}
	}
	return []byte(string(utf16.Decode(units))), nil
}
