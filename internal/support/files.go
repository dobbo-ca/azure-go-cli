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
