package pipelines

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// runsArtifactColumns is transform_runs_artifact_table_output (_format.py:151-155).
var runsArtifactColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Name", Field: "name"},
	{Header: "Type", Field: "resource.type"},
}

func newRunsArtifactCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Manage artifacts associated with a run.",
	}
	cmd.AddCommand(newRunsArtifactListCmd())
	cmd.AddCommand(newRunsArtifactDownloadCmd())
	cmd.AddCommand(newRunsArtifactUploadCmd())
	return cmd
}

func newRunsArtifactListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List artifacts associated with a run.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsArtifactList(context.Background(), cmd)
		},
	}
	cmd.Flags().Int("run-id", 0, "ID of the run that the artifact is associated to.")
	cmd.MarkFlagRequired("run-id")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	return cmd
}

func runRunsArtifactList(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	runID, _ := cmd.Flags().GetInt("run-id")

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var artifacts []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/builds/" + strconv.Itoa(runID) + "/artifacts",
		APIVersion: "5.0",
	}, &artifacts); err != nil {
		return fmt.Errorf("failed to list artifacts: %w", err)
	}

	return ado.Print(cmd, artifacts, runsArtifactColumns...)
}

// --- download / upload -----------------------------------------------------
//
// Deviation, flagged per task rules as a genuine design decision rather than
// a mechanical translation: Python's run_artifact_download/upload
// (dev/pipelines/runs_artifacts.py) never talk REST directly. They download
// a native "artifacttool" binary and shell out to it (dev/common/artifacttool.py),
// speaking a dedup/chunked content-addressable protocol with no public REST
// equivalent. This port is not allowed to invoke any external binary other
// than git (task ground rules), so artifacttool is out.
//
// Instead these two commands talk to the classic FileContainer REST API,
// which is what "runs artifact list"'s `resource.type` (Container/FilePath)
// already implies these artifacts are backed by, and is the same API the
// pre-artifacttool "PublishBuildArtifacts" build task used:
//   - GET  {org}/_apis/resources/Containers?artifactUris=vstfs:///Build/Build/{runId}
//     to find the run's container (devops_sdk file_container_client.get_containers).
//   - PUT  {org}/_apis/resources/Containers/{containerId}?itemPath=...  (raw bytes)
//     to upload a file into it.
//   - POST {project}/_apis/build/builds/{runId}/artifacts  (build_client.create_artifact)
//     to associate the uploaded container path as a named artifact.
//   - GET  {project}/_apis/build/builds/{runId}/artifacts?artifactName=...&$format=zip
//     to download a Container-type artifact as a zip (documented "Get Artifact" $format=zip).
//
// A second, independent deviation: none of this can go through ado.Client.
// Do/List only carry JSON bodies, and DoRaw — documented in
// foundation-spec.md §2.3 as the method for non-JSON payloads — was never
// actually implemented in client.go. So below, auth is resolved via
// ado.ResolveAuth (§3.2's AAD-then-PAT precedence), and runsRawDo replicates
// Client.roundTrip's 401 retry-with-fallback (services.py:54-83's
// validate_token_for_instance fallback): an AAD identity that isn't
// entitled to the org 401s using the AAD header, and this retries once with
// the PAT before giving up, same as every other command in this port.

func newRunsArtifactDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a pipeline artifact.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsArtifactDownload(context.Background(), cmd)
		},
	}
	cmd.Flags().Int("run-id", 0, "ID of the run that the artifact is associated to.")
	cmd.MarkFlagRequired("run-id")
	cmd.Flags().String("artifact-name", "", "Name of the artifact to download.")
	cmd.MarkFlagRequired("artifact-name")
	cmd.Flags().String("path", "", "Path to download the artifact into.")
	cmd.MarkFlagRequired("path")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	return cmd
}

func runRunsArtifactDownload(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	runID, _ := cmd.Flags().GetInt("run-id")
	artifactName, _ := cmd.Flags().GetString("artifact-name")
	destPath, _ := cmd.Flags().GetString("path")

	auth, fallbackAuth, err := ado.ResolveAuth(ctx, dctx.Org)
	if err != nil {
		return err
	}

	reqURL := strings.TrimRight(dctx.Org, "/") + "/" + url.PathEscape(dctx.Project) +
		"/_apis/build/builds/" + strconv.Itoa(runID) + "/artifacts"
	q := url.Values{}
	q.Set("api-version", "5.0")
	q.Set("artifactName", artifactName)
	q.Set("$format", "zip")

	body, err := runsRawDo(ctx, http.MethodGet, reqURL+"?"+q.Encode(), auth, fallbackAuth, nil, "")
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}

	if err := runsExtractZip(body, destPath); err != nil {
		return fmt.Errorf("failed to extract artifact: %w", err)
	}

	// artifacttool prints its own JSON summary to stdout, which Python
	// parses and returns verbatim (or None on a parse failure). We have no
	// equivalent payload, so report what we actually did; no table
	// transformer is registered for this command in Python either
	// (commands.py:144), so table mode falls back to JSON here too.
	return ado.Print(cmd, map[string]any{"artifactName": artifactName, "path": destPath})
}

// runsExtractZip extracts a zip archive (Container artifacts download as a
// zip) into dir, guarding against zip-slip path traversal.
func runsExtractZip(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("response is not a valid zip archive: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	for _, f := range zr.File {
		target := filepath.Join(dir, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) && target != filepath.Clean(dir) {
			return fmt.Errorf("zip entry %q escapes destination directory", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func newRunsArtifactUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a pipeline artifact.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsArtifactUpload(context.Background(), cmd)
		},
	}
	cmd.Flags().Int("run-id", 0, "ID of the run that the artifact is associated to.")
	cmd.MarkFlagRequired("run-id")
	cmd.Flags().String("artifact-name", "", "Name of the artifact to upload.")
	cmd.MarkFlagRequired("artifact-name")
	cmd.Flags().String("path", "", "Path to upload the artifact from.")
	cmd.MarkFlagRequired("path")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	return cmd
}

func runRunsArtifactUpload(ctx context.Context, cmd *cobra.Command) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}
	runID, _ := cmd.Flags().GetInt("run-id")
	artifactName, _ := cmd.Flags().GetString("artifact-name")
	srcPath, _ := cmd.Flags().GetString("path")

	auth, fallbackAuth, err := ado.ResolveAuth(ctx, dctx.Org)
	if err != nil {
		return err
	}

	containerID, err := runsFindContainer(ctx, dctx.Org, auth, fallbackAuth, runID)
	if err != nil {
		return err
	}

	files, err := runsCollectFiles(srcPath)
	if err != nil {
		return err
	}

	containerURL := strings.TrimRight(dctx.Org, "/") + "/_apis/resources/Containers/" + strconv.Itoa(containerID)
	for _, f := range files {
		data, err := os.ReadFile(f.abs)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f.abs, err)
		}
		q := url.Values{}
		q.Set("api-version", "5.0-preview.4")
		q.Set("itemPath", artifactName+"/"+f.rel)
		headers := map[string]string{
			"Content-Range": fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)),
		}
		if _, err := runsRawDo(ctx, http.MethodPut, containerURL+"?"+q.Encode(), auth, fallbackAuth, data, "", headers); err != nil {
			return fmt.Errorf("failed to upload %s: %w", f.rel, err)
		}
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     "POST",
		Scope:      dctx.Project,
		Path:       "build/builds/" + strconv.Itoa(runID) + "/artifacts",
		APIVersion: "5.0",
		Body: map[string]any{
			"name": artifactName,
			"resource": map[string]any{
				"type": "Container",
				"data": fmt.Sprintf("#/%d/%s", containerID, artifactName),
			},
		},
	}, &result); err != nil {
		return fmt.Errorf("failed to associate artifact: %w", err)
	}

	// No table transformer registered for this command in Python either
	// (commands.py:146).
	return ado.Print(cmd, result)
}

type runsUploadFile struct {
	abs string // absolute local path to read
	rel string // path under the artifact name to write to, "/" separated
}

// runsCollectFiles resolves path into the list of files to upload: itself,
// if it is a regular file; every regular file beneath it (relative paths
// preserved), if it is a directory.
func runsCollectFiles(path string) ([]runsUploadFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if !info.IsDir() {
		return []runsUploadFile{{abs: path, rel: filepath.Base(path)}}, nil
	}

	var files []runsUploadFile
	err = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		files = append(files, runsUploadFile{abs: p, rel: filepath.ToSlash(rel)})
		return nil
	})
	return files, err
}

// runsFindContainer looks up the file container associated with runID via
// GetContainers(artifactUris=vstfs:///Build/Build/{runId}). Deviation: this
// only finds a container that already exists (e.g. from a prior publish in
// the same run); it does not attempt to create one from scratch, since no
// verified "create container" route was found in the vendored SDK.
func runsFindContainer(ctx context.Context, org, auth, fallbackAuth string, runID int) (int, error) {
	reqURL := strings.TrimRight(org, "/") + "/_apis/resources/Containers"
	q := url.Values{}
	q.Set("api-version", "5.0-preview.4")
	q.Set("artifactUris", fmt.Sprintf("vstfs:///Build/Build/%d", runID))

	body, err := runsRawDo(ctx, http.MethodGet, reqURL+"?"+q.Encode(), auth, fallbackAuth, nil, "")
	if err != nil {
		return 0, fmt.Errorf("failed to look up file container for run %d: %w", runID, err)
	}

	var page struct {
		Count int               `json:"count"`
		Value []json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return 0, fmt.Errorf("failed to parse container list: %w", err)
	}
	if len(page.Value) == 0 {
		return 0, fmt.Errorf("no file container found for run %d; this port cannot create a fresh one, "+
			"only upload into a container an earlier publish step already created", runID)
	}

	var first struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(page.Value[0], &first); err != nil {
		return 0, fmt.Errorf("failed to parse container: %w", err)
	}
	return first.ID, nil
}

// ponytail: runsRawDo/runsRawDoOnce still duplicate ado.Client's transport
// (retry-on-401, error shaping) because Request has no absolute-URL / raw-body
// escape hatch and no way to add a Content-Range header. Upgrade path: add
// RawBody/Headers fields plus a bytes-returning Do to ado.Client and delete
// both functions -- deferred here only because it changes the failure-path
// error text these commands print today.
//
// runsRawDo sends one raw HTTP request with an Azure DevOps Basic auth
// header and returns the 2xx response body, retrying once with
// fallbackAuth (e.g. a PAT) if auth (e.g. an AAD token) gets a 401 —
// mirrors ado.Client's own 401 retry-with-fallback (see the design-decision
// comment above this section). fallbackAuth == "" or == auth disables the
// retry. contentType, when non-empty, sets Content-Type; extraHeaders
// (variadic so GET/DELETE-style callers can omit it) are applied after it.
func runsRawDo(ctx context.Context, method, rawURL, auth, fallbackAuth string, body []byte, contentType string, extraHeaders ...map[string]string) ([]byte, error) {
	respBody, status, err := runsRawDoOnce(ctx, method, rawURL, auth, body, contentType, extraHeaders...)
	if status == http.StatusUnauthorized && fallbackAuth != "" && fallbackAuth != auth {
		respBody, _, err = runsRawDoOnce(ctx, method, rawURL, fallbackAuth, body, contentType, extraHeaders...)
	}
	return respBody, err
}

// runsRawDoOnce is the single-attempt HTTP round trip runsRawDo retries.
// status is 0 when the request never got a response (network/build error).
func runsRawDoOnce(ctx context.Context, method, rawURL, auth string, body []byte, contentType string, extraHeaders ...map[string]string) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", auth)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, h := range extraHeaders {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}

	logger.Debug("%s %s", method, rawURL) // never log the Authorization header

	httpClient := &http.Client{Timeout: 100 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("%d response from %s: %s", resp.StatusCode, rawURL, strings.TrimSpace(string(respBody)))
	}
	return respBody, resp.StatusCode, nil
}
