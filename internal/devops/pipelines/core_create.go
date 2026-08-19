package pipelines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

const coreGithubRepoType = "github"
const coreAzureRepoType = "TfsGit"

func coreNewCreateCmd() *cobra.Command {
	var name, description, repository, branch, ymlPath, repositoryType, serviceConnection, queueID, folderPath string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Azure Pipeline (YAML based)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			skipFirstRun, err := coreResolveSkipFirstRun(cmd)
			if err != nil {
				return err
			}
			return coreRunCreate(context.Background(), cmd, name, description, repository, branch, ymlPath, repositoryType, serviceConnection, queueID, folderPath, skipFirstRun)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().StringVar(&name, "name", "", "Name of the new pipeline.")
	cmd.Flags().StringVar(&description, "description", "", "Description for the new pipeline.")
	cmd.Flags().StringVar(&repository, "repository", "", "Repository for which the pipeline needs to be configured. Clone URL, repo name, or Owner/RepoName for GitHub. Auto-detected from the local git remote if omitted.")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name for which the pipeline will be configured. Auto-detected from the local checkout if omitted.")
	cmd.Flags().StringVar(&ymlPath, "yml-path", "", "Path of the pipelines yaml file in the repo (if yaml is already present in the repo).")
	cmd.Flags().StringVar(&ymlPath, "yaml-path", "", "Alias for --yml-path.")
	cmd.Flags().StringVar(&repositoryType, "repository-type", "", "Type of repository. Auto-detected from the remote url if omitted. Allowed values: tfsgit, github.")
	cmd.Flags().StringVar(&serviceConnection, "service-connection", "", "Id of the service connection for the repository, for a GitHub repository. Not required for Azure Repos.")
	cmd.Flags().StringVar(&queueID, "queue-id", "", "Id of the queue in the available agent pools. Auto-detected if not specified.")
	agentpoolAddThreeStateFlag(cmd, "skip-first-run", "Do not trigger the first run of the pipeline. Command returns the pipeline instead of a run.")
	agentpoolAddThreeStateFlag(cmd, "skip-run", "Alias for --skip-first-run.")
	cmd.Flags().StringVar(&folderPath, "folder-path", "", `Path of the folder where the pipeline needs to be created. Default is root folder. e.g. "user1/test_pipelines"`)
	cmd.MarkFlagRequired("name")

	return cmd
}

// coreResolveSkipFirstRun ports arguments.py:84-85's
// get_three_state_flag() on --skip-first-run/--skip-run: unset means false
// (trigger the first run), an explicit --skip-first-run=false/--skip-run=false
// means false too, and either flag bare or =true means true.
func coreResolveSkipFirstRun(cmd *cobra.Command) (bool, error) {
	v, err := agentpoolThreeState(cmd, "skip-first-run")
	if err != nil {
		return false, err
	}
	if v == nil {
		v, err = agentpoolThreeState(cmd, "skip-run")
		if err != nil {
			return false, err
		}
	}
	return v != nil && *v, nil
}

// coreRunCreate ports pipeline_create.py:49-155 pipeline_create, minus the
// fully-interactive template-picker branch (no --yml-path: prompts to pick a
// recommended YAML, open it in $EDITOR, then git-push the generated file —
// pipeline_create.py:277-376, requires a TTY end to end and is not
// scriptable). --yml-path is treated as effectively required, per the
// surface spec's own guidance ("a port MUST support it as effectively
// required for CI use"); this is the one deliberate scope-cut in this file,
// called out again in the report.
func coreRunCreate(ctx context.Context, cmd *cobra.Command, name, description, repository, branch, ymlPath, repositoryType, serviceConnection, queueID, folderPath string, skipFirstRun bool) error {
	if ymlPath == "" {
		return fmt.Errorf("--yml-path is required: interactive pipeline template selection (prompting to pick/edit a recommended YAML and push it to the repo) is not supported by this CLI")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreCreate(ctx, cmd, dctx, name, description, repository, branch, ymlPath, repositoryType, serviceConnection, queueID, folderPath, skipFirstRun)
}

// coreCreate does the actual client calls, split out from coreRunCreate for
// testability (see coreList's doc comment in core_list.go).
func coreCreate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, name, description, repository, branch, ymlPath, repositoryType, serviceConnection, queueID, folderPath string, skipFirstRun bool) error {
	// arguments.py:81-82: choices=['tfsgit','github'], type=str.lower —
	// validated/lowercased at parse time, before any of the auto-detection
	// override logic below runs.
	repositoryType, err := coreValidateChoice(repositoryType, "repository-type", coreCreateRepositoryTypeChoices)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	detectFlag, _ := cmd.Flags().GetString("detect")
	detectOn := !strings.EqualFold(detectFlag, "false")

	// dctx.Repo is populated either by --detect (when the current checkout's
	// remote is an Azure DevOps repo) or by echoing back the user's own
	// --repository flag (ado/context.go's resolve reads --repository before
	// falling back to detection) — this stands in for Python's separate
	// resolve_instance_project_and_repo call, which is only consulted when
	// --repository was NOT passed (pipeline_create.py:83-89). Only treat
	// dctx.Repo as a detected Azure repo when the user didn't supply
	// --repository themselves.
	repositoryName := ""
	if !cmd.Flags().Changed("repository") {
		repositoryName = dctx.Repo
	}
	if repositoryName != "" {
		repository = repositoryName
		repositoryType = coreAzureRepoType
	}
	if repository == "" && detectOn {
		if slug, ok := coreDetectGithubRemote(); ok {
			repository = slug
		}
	}
	if repository == "" {
		return fmt.Errorf("the following arguments are required: --repository")
	}
	if repositoryType == "" {
		repositoryType = coreDetectRepositoryType(repository)
	}
	if repositoryType == "" {
		return fmt.Errorf("the following arguments are required: --repository-type. Check command help for valid values")
	}
	if branch == "" && detectOn {
		branch = coreCurrentBranchName()
	}
	if branch == "" {
		return fmt.Errorf("the following arguments are required: --branch")
	}
	if repositoryName == "" {
		if coreIsRepoURL(repository) {
			repositoryName = coreRepoNameFromURL(repository, repositoryType)
		} else {
			repositoryName = repository
		}
	}

	available, err := coreDefinitionNameAvailable(ctx, client, dctx.Project, name, folderPath)
	if err != nil {
		return err
	}
	if !available {
		return fmt.Errorf("pipeline with name %s already exists", name)
	}

	var repoID, repositoryURL, apiURL string
	lowerType := strings.ToLower(repositoryType)
	if lowerType == coreGithubRepoType {
		repoID = repositoryName
		repositoryURL = "https://github.com/" + repositoryName
		apiURL = "https://api.github.com/repos/" + repositoryName
	}
	if lowerType == strings.ToLower(coreAzureRepoType) {
		repoID, err = coreRepositoryIDByName(ctx, client, dctx.Project, repositoryName)
		if err != nil {
			return err
		}
	}

	// Python auto-provisions a GitHub service connection interactively
	// (get_github_service_endpoint, possibly triggering a PAT-creation flow)
	// when omitted; that flow isn't scriptable, so this port requires the
	// flag instead.
	if serviceConnection == "" && lowerType != strings.ToLower(coreAzureRepoType) {
		return fmt.Errorf("--service-connection is required for non-Azure-Repos repositories")
	}

	if queueID == "" {
		autoID, err := coreAutoDetectQueue(ctx, client, dctx.Project)
		if err != nil {
			return err
		}
		if autoID != "" {
			queueID = autoID
		} else {
			logger.Warning("Cannot find a hosted pool queue in the project. Provide a --queue-id in command params.")
		}
	}

	definition := coreBuildDefinitionBody(name, description, repoID, repositoryName, repositoryURL, apiURL, branch, serviceConnection, repositoryType, ymlPath, queueID, folderPath)

	var created map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPost, Scope: dctx.Project, Path: "build/Definitions", APIVersion: "5.1", Body: definition}, &created); err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	logger.Warning("Successfully created a pipeline with Name: %v, Id: %v.", created["name"], created["id"])

	if skipFirstRun {
		return ado.Print(cmd, created, coreDefinitionColumns([]map[string]any{created})...)
	}

	// pipeline_create.py:154: Build(definition=created_definition,
	// source_branch=queue_branch) — msrest serializes `definition` against
	// the DefinitionReference model, so only its id/name survive onto the
	// wire; build that shape directly instead of resending the whole
	// definition.
	runBody := map[string]any{
		"definition":   map[string]any{"id": created["id"], "name": created["name"]},
		"sourceBranch": branch,
	}
	var build map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPost, Scope: dctx.Project, Path: "build/Builds", APIVersion: "5.1", Body: runBody}, &build); err != nil {
		return fmt.Errorf("failed to queue first run: %w", err)
	}

	return ado.Print(cmd, build, coreRunOrDefinitionColumns(build)...)
}

// coreDefinitionNameAvailable ports pipeline_create.py:202-208
// validate_name_is_available.
func coreDefinitionNameAvailable(ctx context.Context, client *ado.Client, project, name, folderPath string) (bool, error) {
	q := url.Values{}
	q.Set("name", name)
	if p := coreFixPathForAPI(folderPath); p != "" {
		q.Set("path", p)
	}

	var defs []map[string]any
	if err := client.List(ctx, ado.Request{Scope: project, Path: "build/Definitions", APIVersion: "5.1", Query: q}, &defs); err != nil {
		return false, fmt.Errorf("failed to check pipeline name availability: %w", err)
	}
	return len(defs) == 0, nil
}

// coreRepositoryIDByName ports pipeline_create.py:503-506
// _get_repository_id_from_name.
func coreRepositoryIDByName(ctx context.Context, client *ado.Client, project, repository string) (string, error) {
	var repo map[string]any
	if err := client.Do(ctx, ado.Request{Scope: project, Path: "git/repositories/" + url.PathEscape(repository), APIVersion: "5.0"}, &repo); err != nil {
		return "", fmt.Errorf("failed to resolve repository %q: %w", repository, err)
	}
	return coreStr(repo["id"]), nil
}

// coreAutoDetectQueue ports pipeline_create.py:508-524
// _get_agent_queue_by_heuristic: prefer "Hosted Ubuntu 1604", else the first
// hosted-pool queue, else the first queue returned. Returns "" (not an
// error) when there are no queues at all.
func coreAutoDetectQueue(ctx context.Context, client *ado.Client, project string) (string, error) {
	var queues []map[string]any
	if err := client.List(ctx, ado.Request{Scope: project, Path: "distributedtask/queues", APIVersion: "5.1-preview.1"}, &queues); err != nil {
		return "", fmt.Errorf("failed to list agent queues: %w", err)
	}
	if len(queues) == 0 {
		return "", nil
	}

	chosen := queues[0]
	foundHosted := false
	for _, q := range queues {
		if name, _ := q["name"].(string); name == "Hosted Ubuntu 1604" {
			chosen = q
			break
		}
		if !foundHosted {
			if pool, ok := q["pool"].(map[string]any); ok {
				if hosted, _ := pool["isHosted"].(bool); hosted {
					chosen = q
					foundHosted = true
				}
			}
		}
	}
	return coreStr(chosen["id"]), nil
}

// coreBuildDefinitionBody ports pipeline_create.py:458-486
// _create_pipeline_build_object.
func coreBuildDefinitionBody(name, description, repoID, repoName, repositoryURL, apiURL, branch, serviceConnection, repositoryType, ymlPath, queueID, path string) map[string]any {
	repo := map[string]any{}
	if repoID != "" {
		repo["id"] = repoID
	}
	if repoName != "" {
		repo["name"] = repoName
	}
	if repositoryURL != "" {
		repo["url"] = repositoryURL
	}
	if branch != "" {
		repo["defaultBranch"] = branch
	}
	if serviceConnection != "" {
		repo["properties"] = map[string]any{
			"connectedServiceId": serviceConnection,
			"defaultBranch":      branch,
			"apiUrl":             apiURL,
		}
	}
	// pipeline_create.py:479-482: "Hack to avoid the case sensitive GitHub
	// type for service hooks."
	if strings.EqualFold(repositoryType, coreGithubRepoType) {
		repo["type"] = "GitHub"
	} else {
		repo["type"] = repositoryType
	}

	def := map[string]any{
		"name":       name,
		"repository": repo,
		"process":    map[string]any{"yamlFilename": ymlPath, "type": 2},
		"queue":      map[string]any{},
		"triggers":   coreDefinitionTriggers(repositoryType),
	}
	if description != "" {
		def["description"] = description
	}
	if path != "" {
		def["path"] = path
	}
	if queueID != "" {
		q := map[string]any{}
		if n, err := strconv.Atoi(queueID); err == nil {
			q["id"] = n
		} else {
			q["id"] = queueID
		}
		def["queue"] = q
	}
	return def
}

// coreDefinitionTriggers ports pipeline_create.py:378-383 _get_pipelines_trigger.
func coreDefinitionTriggers(repositoryType string) []map[string]any {
	if strings.EqualFold(repositoryType, coreGithubRepoType) {
		return []map[string]any{
			{"settingsSourceType": 2, "triggerType": 2},
			{"forks": map[string]any{"enabled": "true", "allowSecrets": "false"}, "settingsSourceType": 2, "triggerType": "pullRequest"},
		}
	}
	return []map[string]any{{"settingsSourceType": 2, "triggerType": 2}}
}

// coreDetectRepositoryType ports pipeline_create.py:267-273 try_get_repository_type.
func coreDetectRepositoryType(repoURL string) string {
	if strings.Contains(repoURL, "https://github.com") {
		return coreGithubRepoType
	}
	if strings.Contains(repoURL, "dev.azure.com") || strings.Contains(repoURL, ".visualstudio.com") {
		return coreAzureRepoType
	}
	return ""
}

// coreIsRepoURL ports pipeline_create.py:212-215 is_valid_url.
func coreIsRepoURL(u string) bool {
	return strings.Contains(u, "github.com") || strings.Contains(u, "visualstudio.com") || strings.Contains(u, "dev.azure.com")
}

// coreRepoNameFromURL ports pipeline_create.py:222-242 _get_repo_name_from_repo_url.
func coreRepoNameFromURL(repoURL, repositoryType string) string {
	if strings.EqualFold(repositoryType, coreGithubRepoType) {
		u, err := url.Parse(repoURL)
		if err == nil && u.Scheme == "https" && u.Host == "github.com" {
			p := strings.Trim(u.Path, "/")
			return strings.TrimSuffix(p, ".git")
		}
	}
	if strings.EqualFold(repositoryType, coreAzureRepoType) {
		parts := strings.Split(repoURL, "/")
		for i, part := range parts {
			if (strings.Contains(part, "visualstudio.com") || strings.Contains(part, "dev.azure.com")) && len(parts) > i+4 {
				return parts[i+4]
			}
		}
	}
	return ""
}

// coreCurrentBranchName ports git.py:45-56 get_current_branch_name.
func coreCurrentBranchName() string {
	out, err := exec.Command("git", "symbolic-ref", "--short", "-q", "HEAD").Output()
	if err != nil {
		logger.Debug("failed to detect current branch: %v", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// coreDetectGithubRemote is a simplified, direct-URL-parsing stand-in for
// Python's get_remote_url(is_github_url_candidate) (git.py:59-69,
// pipeline_create.py:208-220): prefer origin's push URL if it's a GitHub
// remote, else the first push remote that is. Unlike Python it doesn't
// handle ssh:// prefix injection, custom ports, or on-prem SSH hosts —
// deliberately simplified, since it exists only for a local dev convenience
// path (pipeline_create's fallback repository auto-detect for GitHub repos)
// and is not itself part of Azure DevOps org/project/repo resolution
// (unlike ado.detectFromGitRemote, which this does not duplicate).
func coreDetectGithubRemote() (string, bool) {
	out, err := exec.Command("git", "remote", "-v").Output()
	if err != nil {
		return "", false
	}

	remotes := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		remotes[fields[0]+fields[2]] = fields[1]
	}

	if u, ok := remotes["origin(push)"]; ok && strings.Contains(u, "github.com") {
		if slug := coreParseGithubRemote(u); slug != "" {
			return slug, true
		}
	}
	for k, u := range remotes {
		if k == "origin(push)" || !strings.HasSuffix(k, "(push)") {
			continue
		}
		if strings.Contains(u, "github.com") {
			if slug := coreParseGithubRemote(u); slug != "" {
				return slug, true
			}
		}
	}
	return "", false
}

// coreParseGithubRemote extracts "owner/repo" from an https:// or git@ style
// GitHub remote URL.
func coreParseGithubRemote(remoteURL string) string {
	i := strings.Index(remoteURL, "github.com")
	if i < 0 {
		return ""
	}
	rest := remoteURL[i+len("github.com"):]
	rest = strings.TrimPrefix(rest, ":")
	rest = strings.TrimPrefix(rest, "/")
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ".git")
	if rest == "" {
		return ""
	}
	return rest
}
