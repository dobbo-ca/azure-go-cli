package support

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChunkPlan(t *testing.T) {
	for _, c := range [][3]int{{1, 1, 1}, {2621440, 2621440, 1}, {2621441, 2621440, 2}, {5242880, 2621440, 2}} {
		size, chunks := chunkPlan(c[0])
		require.Equal(t, c[1], size, c[0])
		require.Equal(t, c[2], chunks, c[0])
	}
}

func writeFile(t *testing.T, name string, size int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, make([]byte, size), 0o600))
	return p
}

func TestReadUploadRejects(t *testing.T) {
	cases := []struct{ name, path, want string }{
		{"missing", filepath.Join(t.TempDir(), "nope.txt"), "File does not exist!"},
		{"bat", writeFile(t, "a.bat", 5), "The file extension is not supported."},
		{"upper exe", writeFile(t, "RUN.EXE", 5), "The file extension is not supported."},
		{"long stem", writeFile(t, strings.Repeat("a", 111)+".txt", 5), "File name should not be more than 110 characters."},
		{"empty", writeFile(t, "e.txt", 0), "The file must be between 1 and 5242880 bytes"},
		{"too big", writeFile(t, "big.txt", 5242881), "The file must be between 1 and 5242880 bytes"},
	}
	for _, c := range cases {
		_, err := readUpload(c.path)
		require.EqualError(t, err, c.want, c.name)
	}
}

func TestReadUploadOK(t *testing.T) {
	u, err := readUpload(writeFile(t, "notes.txt", 10))
	require.NoError(t, err)
	require.Equal(t, "notes.txt", u.Name)
	require.Len(t, u.Data, 10)

	u, err = readUpload(writeFile(t, "report (1).pdf", 3))
	require.NoError(t, err)
	require.Equal(t, "report (1).pdf", u.Name)
}

func TestUploadFileChunks(t *testing.T) {
	f := newFake()
	require.NoError(t, uploadFile(context.Background(), f, "ws", upload{Name: "f.bin", Data: make([]byte, 3000000)}))
	d := f.files["ws"][0]
	require.EqualValues(t, 3000000, *d.Properties.FileSize)
	require.EqualValues(t, 2621440, *d.Properties.ChunkSize)
	require.EqualValues(t, 2, *d.Properties.NumberOfChunks)
	require.Len(t, f.chunks, 2)
	for i, want := range []int{2621440, 378560} {
		require.EqualValues(t, i, f.chunks[i].index)
		raw, err := base64.StdEncoding.DecodeString(f.chunks[i].content)
		require.NoError(t, err)
		require.Len(t, raw, want)
	}
}
