package support

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
)

const (
	maxChunkSize = 2621440 // 2.5 MiB
	maxFileSize  = 5242880
)

// workspaceNamePattern is the Python file-workspace name pattern; file names have none.
var workspaceNamePattern = regexp.MustCompile(`^[0-9a-zA-Z_\-. ]+$`)

var blockedExtensions = map[string]bool{}

func init() {
	for _, e := range strings.Fields(".bat .cmd .exe .ps1 .js .vbs .com .lnk .reg .bin .cpl .inf .ins .isu .job .jse .msi .msp .paf .pif .rgs .scr .sct .vbe .vb .ws .wsf .wsh") {
		blockedExtensions[e] = true
	}
}

type upload struct {
	Name string
	Data []byte
}

// readUpload validates a local file against the support upload rules and reads it.
func readUpload(path string) (upload, error) {
	info, err := os.Stat(path)
	if err != nil {
		return upload{}, errors.New("File does not exist!")
	}
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if blockedExtensions[strings.ToLower(ext)] {
		return upload{}, errors.New("The file extension is not supported.")
	}
	if utf8.RuneCountInString(strings.TrimSuffix(name, ext)) > 110 {
		return upload{}, errors.New("File name should not be more than 110 characters.")
	}
	if info.Size() < 1 || info.Size() > maxFileSize {
		return upload{}, fmt.Errorf("The file must be between 1 and %d bytes", maxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return upload{}, err
	}
	return upload{Name: name, Data: data}, nil
}

// chunkPlan returns the chunk size and chunk count for a file of the given size.
func chunkPlan(size int) (chunkSize, chunks int) {
	chunkSize = min(maxChunkSize, size)
	return chunkSize, (size + chunkSize - 1) / chunkSize
}

// uploadFile creates the file in the workspace, then sends its chunks in order.
func uploadFile(ctx context.Context, c Client, workspace string, u upload) error {
	size := len(u.Data)
	chunkSize, chunks := chunkPlan(size)
	fs, cs, n := int32(size), int32(chunkSize), int32(chunks)
	_, err := c.CreateFile(ctx, workspace, u.Name, armsupport.FileDetails{
		Properties: &armsupport.FileDetailsProperties{FileSize: &fs, ChunkSize: &cs, NumberOfChunks: &n},
	})
	if err != nil {
		return err
	}
	for i := 0; i < chunks; i++ {
		part := u.Data[i*chunkSize : min((i+1)*chunkSize, size)]
		if err := c.UploadChunk(ctx, workspace, u.Name, int32(i), base64.StdEncoding.EncodeToString(part)); err != nil {
			return err
		}
	}
	return nil
}

func newFileCmd(tenantLevel bool) *cobra.Command {
	upload := &cobra.Command{
		Use:   "upload",
		Short: "Upload a file to a file workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := readUpload(str(cmd, "file-path"))
			if err != nil {
				return err
			}
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			if err := uploadFile(cmd.Context(), c, str(cmd, "file-workspace-name"), u); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "File '%s' has been successfully uploaded.\n", u.Name)
			return nil
		},
	}
	addStr(upload, "file-workspace-name", "File workspace name.", true)
	addStr(upload, "file-path", "Path of the file to upload.", true)

	list := &cobra.Command{
		Use:   "list",
		Short: "List the files in a file workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			max, tok, err := paging(cmd)
			if err != nil {
				return err
			}
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			items, next, err := c.ListFiles(cmd.Context(), str(cmd, "file-workspace-name"), max, tok)
			if err != nil {
				return err
			}
			return printPage(cmd, items, next)
		},
	}
	addStr(list, "file-workspace-name", "File workspace name.", true)
	addListFlags(list)

	show := &cobra.Command{
		Use:   "show",
		Short: "Show a file in a file workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			f, err := c.GetFile(cmd.Context(), str(cmd, "file-workspace-name"), str(cmd, "file-name"))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, f)
		},
	}
	addStr(show, "file-workspace-name", "File workspace name.", true)
	addStr(show, "file-name", "File name.", true)

	return group("file", "Manage files in a file workspace", upload, list, show)
}

func newFileWorkspaceCmd(tenantLevel bool) *cobra.Command {
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a file workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := str(cmd, "file-workspace-name")
			if !workspaceNamePattern.MatchString(name) {
				return fmt.Errorf("file workspace name %q is invalid: it must match %s", name, workspaceNamePattern)
			}
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			if err := c.CheckName(cmd.Context(), name, typeFileWorkspaces); err != nil {
				return err
			}
			w, err := c.CreateFileWorkspace(cmd.Context(), name)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, w)
		},
	}
	addStr(create, "file-workspace-name", "File workspace name.", true)

	show := &cobra.Command{
		Use:   "show",
		Short: "Show a file workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, tenantLevel)
			if err != nil {
				return err
			}
			w, err := c.GetFileWorkspace(cmd.Context(), str(cmd, "file-workspace-name"))
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, w)
		},
	}
	addStr(show, "file-workspace-name", "File workspace name.", true)

	return group("file-workspace", "Manage file workspaces", create, show)
}
