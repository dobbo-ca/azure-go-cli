# az support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `az support`: flag commands matching the Python `support` extension, plus a full-screen TUI to list, read, reply to and create Azure support tickets for any subscription or tenant.

**Architecture:** One flat package, `internal/support`. A `Client` interface wraps the armsupport clients and has two implementations, `subscriptionClient` and `tenantClient` (no-subscription). The cobra flag commands and the bubbletea TUI both call shared helpers through that interface: `createTicket`, `uploadFile`, `loadCatalog` and `rank`. The TUI lives in `tui_*.go` files in the same package. A `tui/` subpackage would need to import `support` while `support` imports it to launch the TUI, which is a cycle.

**Tech Stack:** Go, cobra, `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2 v2.0.0` (API 2024-04-01), `armresources v1.2.0` (already in go.mod), `charm.land/bubbletea/v2 v2.1.2`, `charm.land/bubbles/v2 v2.2.2`, `charm.land/lipgloss/v2 v2.1.1`, `golang.org/x/text` (`language`, `language/display`), embedded CLDR `release-49` XML.

**Spec:** `docs/superpowers/specs/2026-11-23-az-support-design.md`

## Global Constraints

- SDK: `armsupport/v2 v2.0.0` only. Do not use classifier or preview APIs.
- Flag names, required flags and enums must match the Python extension exactly. Task 4's parity table is the source; when in doubt, read `azext_support/aaz/latest/support/**/_*.py` and do not rely on memory.
- With `--no-input`, a non-TTY stdin or stdout, or any subcommand, no TUI starts.
- Nothing the user types is written to disk. The profile is never written: switching scope does not change the default subscription.
- API errors are shown verbatim, in the TUI as well as from the flag commands. Nothing fails silently.
- Severity values are `minimal|moderate|critical|highestcriticalimpact`. Consent is `Yes|No`. Contact method is `email|phone`. The 11 language codes are `en-us zh-hans es-es fr-fr ja-jp ko-kr ru-ru pt-br it-it zh-tw de-de`. Country is ISO 3166-1 alpha-3. Time zone is a Windows zone name.
- A phone number is required when the contact method is `phone` or the severity is `critical` or `highestcriticalimpact`.
- Upload rules (Python `_utils.upload_file`):
  - Blocked extensions: `.bat .cmd .exe .ps1 .js .vbs .com .lnk .reg .bin .cpl .inf .ins .isu .job .jse .msi .msp .paf .pif .rgs .scr .sct .vbe .vb .ws .wsf .wsh`.
  - The stem must be 110 characters or fewer.
  - The size must be between 1 and 5242880 bytes.
  - Chunk size is `min(2621440, size)`. Chunks are base64-encoded and sent in index order (0-based).
- Every ticket this CLI creates gets a file workspace, unless the flag command was given one.
- Ticket list filters are OData: `Status eq 'Open'`, and `CreatedDate ge YYYY-MM-DD`.
- Formatting: the new package uses tabs. Run gofmt only on files under `internal/support/` and on `cmd/az/main.go`, never on the whole tree.
- Build with `make build`, which outputs `bin/az/az`. Tests run with `go test ./internal/support/...`. Tests make no live Azure or Graph calls: fake `Client`, `deps` and `httptest`.
- Adding the charm modules raises the go.mod `go` directive to `1.26.0`, because the charm modules require it. CI uses `go-version-file`. The release workflow pins `go-version: '1.23'` and already relies on toolchain auto-download for `1.25.2`. Leave it as is and say so in the PR body.
- Commits use conventional commits (`feat(support): …`, `test(support): …`, `docs(support): …`).
- In TUI code, do not use `tea.Sequence`: its message type is unexported and the test driver cannot run it. Use `tea.Batch`, or chain through messages.
- Turn off cursor blinking on every `textinput` and `textarea` a screen creates: `st := m.Styles(); st.Cursor.Blink = false; m.SetStyles(st)`. With blinking on, `Focus()` and every cursor move return a Cmd that sleeps 530 ms and yields `cursor.BlinkMsg`, which re-arms itself, so `drive` would crawl to its depth cap.

## Review Focus

1. **A flag-command `--ticket-name` that is not a valid workspace name.** For example `my/ticket` or `tické`, which do not match `^[0-9a-zA-Z_\-. ]+$`. The auto-created workspace should then get a GUID name, not fail after the ticket name check. Task 3: `TestCreateTicketWorkspaceNameFallsBackToGUID`.
2. **A local time zone that CLDR lists under a legacy alias.** Examples are `Asia/Kolkata` and `Europe/Kyiv`, or a macOS `/etc/localtime` link such as `/var/db/timezone/zoneinfo/America/Vancouver`. These should still resolve to a Windows zone and not fall back to the picker. Task 9: `TestWindowsZoneAliases` and `TestIANAFromTZ`.
3. **Switching scope while a fetch for the old scope is still running.** Late results from the old scope must not replace the new scope's ticket list. Task 11 adds a scope generation counter to every async message, and Task 12 adds `TestListIgnoresStaleScopeResults`. The same race one level down (a late result after `f`, or for a ticket the user already left) is covered by `TestListIgnoresStaleFilterResults` and `TestDetailIgnoresOtherTicketLoad`.
4. **An upper-case blocked extension.** For example `RUN.EXE`. It should be rejected before any API call, so before a ticket is created. File names themselves have no pattern (only workspace names do), so `report (1).pdf` is accepted. Task 2: `TestReadUploadRejects` and `TestReadUploadOK`.
5. **Graph `/me` failing or returning partial data.** This covers a 403 in a guest tenant, a null `mail`, and a guest UPN containing `#EXT#`. The contact fields should start empty or fall back as appropriate, and the flow should continue. Task 9: `TestContactFromMe` and `TestFetchMeError`.

---

## File map

All files are in `internal/support/` unless a path is given. Each has a `_test.go` sibling when its task adds tests.

| File | Responsibility |
|---|---|
| `client.go` | `Client` interface, `subscriptionClient`, `tenantClient`, `credentialFor`, `NewClient` |
| `scope.go` | `Scope`, `flagScope`, `startScope`, `commandClient` |
| `tickets.go` | problem-classification ID parsing, service categories, `createTicket`, list filter, technical-resource validation, `tickets` commands |
| `files.go` | `readUpload`, `chunkPlan`, `uploadFile`, `file` and `file-workspace` commands |
| `communication.go` | `communication` and `chat-transcript` commands |
| `services.go` | `services` and `problem-classifications` commands, `catalogRow`, `loadCatalog` |
| `commands.go` | cobra tree, `tuiAllowed`, launching the TUI |
| `search.go` | `rank`, `queryFromTitle` |
| `htmltext.go` | `htmlToText` |
| `tzdata.go`, `cldr/` | embedded CLDR files, `windowsZone`, `windowsZoneNames`, `ianaFromTZ` |
| `contact.go` | `me`, `fetchMe`, `contactDefaults`, `contactFromMe`, `supportLanguage`, `countryAlpha3`, `countryOptions` |
| `resources.go` | `resourceRef`, `resourceTypeFilter`, `listResources` |
| `tui_app.go` | `app` root model, `screen`, `deps`, navigation, `runTUI` |
| `tui_picker.go` | `picker`, a filterable single-select list used by every list-like step |
| `tui_scope.go` | scope switcher screen |
| `tui_list.go` | ticket list screen |
| `tui_detail.go` | ticket detail screen, `renderThread` |
| `tui_reply.go` | reply editor screen |
| `tui_create.go` | new-ticket flow screen |
| `fake_test.go` | `fakeClient`, `fakeDeps`, `drive`, `keys` |

---

### Task 1: SDK dependency, Client interface, Scope

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/support/client.go`, `internal/support/scope.go`, `internal/support/fake_test.go`
- Test: `internal/support/client_test.go`, `internal/support/scope_test.go`

**Interfaces:**
- Consumes: `azure.TenantCredential(tenantID string, authRecord azidentity.AuthenticationRecord)` (`pkg/azure/authmode.go:87`), `azure.GetCredential()` (`pkg/azure/client.go:10`), `config.Load()` (`pkg/config/profile.go:87`), `config.GetSubscription(string)` (`:148`), `config.GetTenantID(string)` (`:174`).
- Produces:

```go
type Client interface {
	CheckName(ctx context.Context, name string, typ armsupport.Type) error // nil = available; else errors.New(*Message)
	CreateTicket(ctx context.Context, name string, t armsupport.TicketDetails) (armsupport.TicketDetails, error) // BeginCreate + PollUntilDone
	GetTicket(ctx context.Context, name string) (armsupport.TicketDetails, error)
	ListTickets(ctx context.Context, filter string) ([]*armsupport.TicketDetails, error) // all pages; "" = no filter
	UpdateTicket(ctx context.Context, name string, u armsupport.UpdateSupportTicket) (armsupport.TicketDetails, error)
	CheckCommunicationName(ctx context.Context, ticket, name string) error
	CreateCommunication(ctx context.Context, ticket, name string, c armsupport.CommunicationDetails) (armsupport.CommunicationDetails, error)
	GetCommunication(ctx context.Context, ticket, name string) (armsupport.CommunicationDetails, error)
	ListCommunications(ctx context.Context, ticket, filter string) ([]*armsupport.CommunicationDetails, error) // all pages
	GetChatTranscript(ctx context.Context, ticket, name string) (armsupport.ChatTranscriptDetails, error)
	ListChatTranscripts(ctx context.Context, ticket string) ([]*armsupport.ChatTranscriptDetails, error)
	CreateFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error)
	GetFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error)
	CreateFile(ctx context.Context, workspace, name string, f armsupport.FileDetails) (armsupport.FileDetails, error)
	UploadChunk(ctx context.Context, workspace, name string, index int32, content string) error
	GetFile(ctx context.Context, workspace, name string) (armsupport.FileDetails, error)
	ListFiles(ctx context.Context, workspace string) ([]*armsupport.FileDetails, error)
	ListServices(ctx context.Context) ([]*armsupport.Service, error)
	GetService(ctx context.Context, name string) (armsupport.Service, error)
	ListProblemClassifications(ctx context.Context, service string) ([]*armsupport.ProblemClassification, error)
	GetProblemClassification(ctx context.Context, service, name string) (armsupport.ProblemClassification, error)
}

const typeFileWorkspaces = armsupport.Type("Microsoft.Support/fileWorkspaces") // not in the SDK enum; Python uses it

type Scope struct{ TenantID, TenantName, SubscriptionID, SubscriptionName string }
func (s Scope) TenantLevel() bool // SubscriptionID == ""
func (s Scope) Label() string     // header text
func credentialFor(tenantID string) (azcore.TokenCredential, error) // "" -> azure.GetCredential()
func NewClient(s Scope) (Client, error)
func newClient(s Scope, cred azcore.TokenCredential, opts *arm.ClientOptions) (Client, error)
func flagScope(cmd *cobra.Command) (Scope, error)  // --subscription or default; tenant via GetTenantID, "" if not in profile
func commandClient(cmd *cobra.Command, tenantLevel bool) (c Client, sessionSub string, err error)
var commandClientFn = commandClient // test seam; every command calls commandClientFn

// fake_test.go
type fakeClient struct { /* in-memory tickets, comms, workspaces, files, services, problems; calls []string; per-method err fields */ }
```

`subscriptionClient` wraps `TicketsClient`, `CommunicationsClient`, `ChatTranscriptsClient`, `FileWorkspacesClient` and `FilesClient`. `tenantClient` wraps their `*NoSubscription*` equivalents. Both embed one `catalog` struct holding `*ServicesClient` and `*ProblemClassificationsClient`. Both `CheckName` implementations call `Tickets[NoSubscription]Client.CheckNameAvailability`, and both `CheckCommunicationName` implementations call `Communications[NoSubscription]Client.CheckNameAvailability` with `TypeMicrosoftSupportCommunications`. `UploadChunk` sends `armsupport.UploadFile{ChunkIndex, Content}`.

`Scope.Label()` returns `"<TenantName|TenantID> · <SubscriptionName> (<SubscriptionID>)"`, or `"<TenantName|TenantID> · tenant-level (no subscription)"`.

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2@v2.0.0`
Expected: go.mod lists `armsupport/v2 v2.0.0`.

- [ ] **Step 2: Write the failing tests**

`client_test.go` uses `httptest.NewTLSServer` with `arm.ClientOptions{ClientOptions: policy.ClientOptions{Cloud: cloud.Configuration{Services: map[cloud.ServiceName]cloud.ServiceConfiguration{cloud.ResourceManager: {Endpoint: srv.URL, Audience: "https://management.azure.com"}}}, Transport: srv.Client()}}` and a static fake `azcore.TokenCredential`. Fake credentials are hand-written, following `internal/pim/tokencred_test.go:12`.

```go
func TestSubscriptionClientListTicketsPaths(t *testing.T) {
	// server: GET /subscriptions/sub1/providers/Microsoft.Support/supportTickets returns 1 ticket + nextLink; page 2 returns 1
	got, err := c.ListTickets(ctx, "Status eq 'Open'")
	require.NoError(t, err); require.Len(t, got, 2)
	require.Equal(t, "Status eq 'Open'", firstRequest.URL.Query().Get("$filter"))
}
func TestTenantClientListTicketsPath(t *testing.T) // path == "/providers/Microsoft.Support/supportTickets"
func TestCheckNameUnavailableReturnsMessage(t *testing.T) // body {"nameAvailable":false,"message":"Name taken"} -> err.Error()=="Name taken"
func TestScopeLabel(t *testing.T) // {TenantID:"t1",SubscriptionID:"s1",SubscriptionName:"Prod"} -> "t1 · Prod (s1)"; {TenantID:"t1",TenantName:"Contoso"} -> "Contoso · tenant-level (no subscription)"
```

- [ ] **Step 3: Run the tests and confirm they fail**

Run: `go test ./internal/support/ -run 'TestSubscriptionClient|TestTenantClient|TestCheckName|TestScopeLabel' -v`
Expected: FAIL (undefined: newClient, Scope).

- [ ] **Step 4: Implement `client.go` and `scope.go`, then write `fake_test.go`**

`fakeClient` implements `Client`:
- Each call appends `"Method:arg"` to `calls`.
- `CreateTicket` stores the details and returns them with `Name` set.
- `UploadChunk` records `(index, content)` in `chunks`.
- `CheckName` fails when the name is in `takenNames`, with the error "Name taken".
- `CheckCommunicationName` also fails when the name is in `takenNames`, with the error "Name taken".
- Each `List*` method records its filter in `lastFilter`.

Add `var _ Client = (*fakeClient)(nil)`.

- [ ] **Step 5: Run the tests and confirm they pass**

Run: `go test ./internal/support/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/support/   # expect no output
git add go.mod go.sum internal/support/
git commit -m "feat(support): add armsupport client interface and scope"
```

---

### Task 2: File upload validation and chunking

**Files:**
- Create: `internal/support/files.go` (helpers; commands come in Task 6)
- Test: `internal/support/files_test.go`

**Interfaces:**
- Consumes: `Client`, `fakeClient`.
- Produces:

```go
const maxChunkSize = 2621440 // 2.5 MiB
const maxFileSize = 5242880
var workspaceNamePattern = regexp.MustCompile(`^[0-9a-zA-Z_\-. ]+$`) // Python file-workspace name pattern; file names have none
type upload struct{ Name string; Data []byte }
func readUpload(path string) (upload, error)
func chunkPlan(size int) (chunkSize, chunks int) // chunkSize=min(maxChunkSize,size); chunks=ceil(size/chunkSize)
func uploadFile(ctx context.Context, c Client, workspace string, u upload) error // CreateFile with FileSize/ChunkSize/NumberOfChunks, then UploadChunk(i, base64.StdEncoding(slice)) in order
```

`readUpload` checks in this order, stopping at the first failure:

| Check | Error (exact text) |
|---|---|
| Path exists (`os.Stat`) | `File does not exist!` |
| Extension, lower-cased with `strings.ToLower(filepath.Ext)`, is not blocked | `The file extension is not supported.` |
| Stem is 110 runes or fewer | `File name should not be more than 110 characters.` |
| Stat size is between 1 and 5242880 bytes | `The file must be between 1 and 5242880 bytes` |

Only then is the file read. The extension check lower-cases the extension, which is stricter than Python's case-sensitive check.

- [ ] **Step 1: Write the failing tests**

```go
func TestChunkPlan(t *testing.T) // 1->(1,1); 2621440->(2621440,1); 2621441->(2621440,2); 5242880->(2621440,2)
func TestReadUploadRejects(t *testing.T) // table, files in t.TempDir(): missing, "a.bat", "RUN.EXE", 111-char stem, empty file, 5242881 bytes -> exact messages above
func TestReadUploadOK(t *testing.T)      // "notes.txt" 10 bytes -> Name "notes.txt", len(Data)==10; "report (1).pdf" accepted (file names have no pattern)
func TestUploadFileChunks(t *testing.T)  // 3,000,000-byte file -> CreateFile{FileSize:3000000,ChunkSize:2621440,NumberOfChunks:2}; chunks[0].index==0 len(decoded)==2621440; chunks[1].index==1 len(decoded)==378560
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run 'TestChunkPlan|TestReadUpload|TestUploadFile' -v` and confirm it FAILS.
- [ ] **Step 3: Implement the helpers in `files.go`**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): add support file upload validation and chunking`

---

### Task 3: Ticket helpers and shared create orchestration

**Files:**
- Create: `internal/support/tickets.go` (helpers only; commands come in Task 5)
- Test: `internal/support/tickets_test.go`

**Interfaces:**
- Consumes: `Client`, `fakeClient` (Task 1); `upload`, `uploadFile`, `workspaceNamePattern` (Task 2).
- Produces:

```go
const (
	billingServiceGUID  = "517f2da6-78fd-0498-4e22-ad26996b1dfc"
	quotaServiceGUID    = "06bfd9d3-516b-d5c6-5802-169c800dec89"
	subMgmtServiceGUID  = "f3dc5421-79ef-1efa-41a5-42bf3cbb52c6"
	errInvalidProblemClassification = "Problem classification id is invalid. Please use 'az support services problem-classifications list' to get the most recent set of problem classification ids for your service."
)
var (
	now     = time.Now       // test seam
	newName = uuid.NewString // test seam
)
func parseProblemClassificationID(id string) (service, problem string, ok bool) // lower-cased; regex ^/providers/microsoft.support/services/(GUID)/problemclassifications/(GUID)$
func serviceIDFor(service string) string // "/providers/Microsoft.Support/services/" + service
func isTechnicalService(service string) bool // not billing/quota/subMgmt, substring match on lower-cased input
func isQuotaService(service string) bool
func defaultListFilter(t time.Time) string // "CreatedDate ge " + t.AddDate(0,0,-7).Format("2006-01-02")
func validateTechnicalResource(resourceID, sessionSub string) error
func needsPhone(method armsupport.PreferredContactMethod, sev armsupport.SeverityLevel) bool
func createTicket(ctx context.Context, c Client, name string, td armsupport.TicketDetails, attach *upload) (armsupport.TicketDetails, error)
```

`validateTechnicalResource` copies the Python messages exactly. `base` is `"Technical resource argument <id> is invalid."`.
- An empty ID is valid and returns nil.
- If `arm.ParseResourceID` fails, return `base`.
- If the subscription is not a GUID (`uuid.Parse`), return `base + "Subscription id <s> is invalid."`.
- If the subscription is not `sessionSub`, return `"<base> <s> does not match with <sessionSub>"`.

`createTicket` runs these steps in order:
1. Parse `td.Properties.ProblemClassificationID`. If it does not parse, return `errInvalidProblemClassification`.
2. Set `ServiceID`.
3. If `ProblemStartTime` is nil, set it to `now().UTC().Truncate(time.Second)`.
4. `CheckName(name, TypeMicrosoftSupportSupportTickets)`.
5. If `FileWorkspaceName` is nil:
   - The workspace name is `name` if it matches `workspaceNamePattern`, otherwise `newName()`.
   - `CheckName(ws, typeFileWorkspaces)`, then `CreateFileWorkspace(ws)`, then set `FileWorkspaceName`.
6. If `attach != nil`, call `uploadFile(ctx, c, ws, *attach)`.
7. Return `CreateTicket(name, td)`.

The function returns on the first error and does not roll back. An empty workspace costs nothing and expires on its own.

- [ ] **Step 1: Write the failing tests**

```go
func TestParseProblemClassificationID(t *testing.T) // valid mixed-case id -> lower-case svc/pc, ok; "/providers/Microsoft.Support/services/x/problemClassifications/y" -> !ok
func TestIsTechnicalService(t *testing.T)            // billing/quota/subMgmt GUIDs -> false; other GUID -> true; isQuotaService(quota)=true
func TestDefaultListFilter(t *testing.T)             // 2026-11-24 -> "CreatedDate ge 2026-11-17"
func TestValidateTechnicalResource(t *testing.T)     // ""->nil; "bad"->"Technical resource argument bad is invalid."; other sub -> "... does not match with <sub>"; same sub -> nil
func TestNeedsPhone(t *testing.T)                    // (email,minimal)=false (phone,minimal)=true (email,critical)=true (email,highestcriticalimpact)=true
func TestCreateTicketOrder(t *testing.T) {
	// fake; name "abc-1"; no workspace; attach small upload
	// calls == [CheckName:abc-1, CheckName:abc-1(fileWorkspaces), CreateFileWorkspace:abc-1, CreateFile:abc-1/f.txt, UploadChunk:..., CreateTicket:abc-1]
	// stored ticket: ServiceID "/providers/Microsoft.Support/services/<svc>", FileWorkspaceName "abc-1", ProblemStartTime == fixed now (UTC, seconds)
}
func TestCreateTicketKeepsGivenWorkspace(t *testing.T)            // FileWorkspaceName "ws1" set -> no CreateFileWorkspace call
func TestCreateTicketWorkspaceNameFallsBackToGUID(t *testing.T)  // name "my/ticket" -> workspace == newName() stub "0000…"
func TestCreateTicketNameTakenStopsEarly(t *testing.T)           // takenNames{"abc-1"} -> error, no CreateFileWorkspace/CreateTicket
func TestCreateTicketInvalidPC(t *testing.T)                     // err.Error()==errInvalidProblemClassification, no calls
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test ./internal/support/ -run 'TestParseProblem|TestIsTechnical|TestDefaultListFilter|TestValidateTechnical|TestNeedsPhone|TestCreateTicket' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement the helpers in `tickets.go`**

- [ ] **Step 4: Run the tests and confirm they pass** (same command; expected PASS)

- [ ] **Step 5: Commit** `feat(support): add ticket create orchestration and validators`

---

### Task 4: Command tree, services commands, registration

**Files:**
- Create: `internal/support/commands.go`, `internal/support/services.go`
- Modify: `cmd/az/main.go` (import `internal/support`, add `support.NewSupportCommand()` to `rootCmd.AddCommand`, around lines 81-112; the list is roughly alphabetical, so put it after `storage.NewStorageCommand()` at line 100)
- Test: `internal/support/commands_test.go`

**Interfaces:**
- Consumes: `commandClientFn`, `output.PrintJSON(cmd, data)` (`pkg/output/output.go:27`).
- Produces:

```go
func NewSupportCommand() *cobra.Command
func newScopeGroup(use string, tenantLevel bool) *cobra.Command // "in-subscription" / "no-subscription"; adds tickets, communication, chat-transcript, file, file-workspace groups (Tasks 5-6 fill them)
func newServicesCmd() *cobra.Command
func tuiAllowed(noInput, stdinTTY, stdoutTTY bool) bool // !noInput && stdinTTY && stdoutTTY
```

- `az support` has local flags `--tenant` and `--no-input`. With no subcommand: if `tuiAllowed(...)` (`term.IsTerminal` on stdin and stdout, as at `internal/pim/prompt.go:29`), call `runTUI(cmd)`. Until Task 11, `runTUI` returns `errors.New("TUI not built yet")`. Otherwise call `cmd.Help()`.
- `az support tui` has the same flags. It returns `errors.New("az support tui needs an interactive terminal")` when the TUI is not allowed.
- Set `SilenceUsage: true` on every command (precedent: `internal/lock/commands.go:27`).
- The services commands are `services list`, `services show --service-name`, `services problem-classifications list --service-name` and `show --service-name --problem-classification-name`. They print through `output.PrintJSON`, and lists print as arrays.

The test file also defines the parity table, which Tasks 5 and 6 extend. Columns: command path, then flags, with `*` meaning required.

| Path | Flags |
|---|---|
| `services show` | `*service-name` |
| `services problem-classifications list` | `*service-name` |
| `services problem-classifications show` | `*service-name *problem-classification-name` |

- [ ] **Step 1: Write the failing tests**

```go
func TestTuiAllowed(t *testing.T) // (false,true,true)=true; (true,true,true)=false; (false,false,true)=false; (false,true,false)=false
func TestFlagParity(t *testing.T) {
	// for each row: cmd,_,err := NewSupportCommand().Find(strings.Fields(path)); require.NoError
	// for each flag: f := cmd.Flags().Lookup(name); require.NotNil
	// required := f.Annotations[cobra.BashCompOneRequiredFlag] != nil; require.Equal(t, wantRequired, required)
}
func TestSupportNoSubcommandNonTTYPrintsHelp(t *testing.T) // cmd.SetArgs([]string{"--no-input"}); out buffer contains "in-subscription"
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run 'TestTuiAllowed|TestFlagParity|TestSupportNoSub' -v` and confirm it FAILS.
- [ ] **Step 3: Implement `commands.go` and `services.go`, and register the command in `cmd/az/main.go`**
- [ ] **Step 4: Run the tests (PASS), then `make build && ./bin/az/az support services --help`, and check the help lists `list`, `show` and `problem-classifications`.**
- [ ] **Step 5: Commit** `feat(support): add az support command tree and services commands`

---

### Task 5: tickets create/list/show/update (both scopes)

**Files:**
- Modify: `internal/support/tickets.go`, `internal/support/commands_test.go` (parity rows)
- Test: `internal/support/tickets_test.go`

**Interfaces:**
- Consumes: `createTicket`, `validateTechnicalResource`, `isTechnicalService`, `parseProblemClassificationID`, `defaultListFilter`, `commandClientFn`.
- Produces:

```go
func newTicketsCmd(tenantLevel bool) *cobra.Command
func ticketFromFlags(cmd *cobra.Command, tenantLevel bool) (armsupport.TicketDetails, error)
func updateFromFlags(cmd *cobra.Command) armsupport.UpdateSupportTicket // only Changed flags; ContactDetails nil unless a --contact-* flag changed
func parseStartTime(s string) (time.Time, error) // RFC3339 | "2006-01-02T15:04:05" (UTC) | "2006-01-02" (UTC)
func parseSecondaryConsent(s string) ([]*armsupport.SecondaryConsent, error) // JSON array [{"type":..,"user-consent":"Yes|No"}], or "@file"
func parseQuotaRequests(s string) ([]*armsupport.QuotaChangeRequest, error)  // JSON array [{"payload":..,"region":..}], or "@file"
```

Parity rows (`*` means required). `<S>` stands for both `in-subscription` and `no-subscription` unless marked.

| Path | Flags |
|---|---|
| `<S> tickets create` | `*ticket-name *advanced-diagnostic-consent *description *problem-classification *severity *title start-time require-24-by-7-response secondary-consent enrollment-id technical-resource *contact-country *contact-first-name *contact-last-name *contact-method *contact-language *contact-timezone *contact-email contact-phone-number contact-additional-emails` |
| `in-subscription tickets create` (in addition) | `file-workspace quota-change-subtype quota-change-version quota-change-requests` |
| `no-subscription tickets create` (in addition) | `file-workspace-name` |
| `<S> tickets update` | `*ticket-name advanced-diagnostic-consent severity status secondary-consent contact-country contact-first-name contact-last-name contact-method contact-language contact-timezone contact-email contact-phone-number contact-additional-emails` |
| `<S> tickets list` | `filter max-items` |
| `<S> tickets show` | `*ticket-name` |

Also add a negative check: `no-subscription tickets create` has no `quota-change-subtype` flag.

Behaviour:
- Enum flags are validated against the exact Python sets. For example, `--severity` accepts `critical|highestcriticalimpact|minimal|moderate`, `--status` accepts `closed|open`, and consent accepts `No|Yes`.
- `--contact-additional-emails` is a `StringSlice`, so values can be comma-separated or the flag repeated.
- `--require-24-by-7-response` is a `Bool`, so it is passed bare or as `=false`.
- Create checks the technical resource only when the service is technical and the flag is set. The comparison uses `sessionSub`, which is the resolved `--subscription` or default for both variants, matching Python's `get_subscription_id`.
- Create passes `attach=nil` to `createTicket` and prints the created ticket.
- List uses `defaultListFilter(now())` only when both `--filter` and `--max-items` are unset (Python `custom.py:163`). It prints an array.
- `--max-items N` (Int) on every list command truncates the result to N items after paging (`ponytail:` all pages are still fetched; stop paging early if a list is ever large enough to matter).
- Show and update print the ticket.

Deviations from Python, which go in the help text:
- The two list-of-object flags take JSON (or `@file`), not aaz shorthand.
- Create also creates a file workspace when none is given.
- `--next-token` is not offered on list commands: the SDK pagers cannot resume from a token. `--max-items` is.

- [ ] **Step 1: Write the failing tests**

Tests swap `commandClientFn` (Task 1) to return a `fakeClient`.

```go
func TestTicketsCreateBuildsDetails(t *testing.T) // args for in-subscription incl. --contact-additional-emails a@x,b@y --require-24-by-7-response --start-time 2026-11-20
	// stored ticket: Title, Description, Severity=moderate, AdvancedDiagnosticConsent=Yes, ContactDetails{Country:"CAN",PreferredContactMethod:email,PreferredTimeZone:"Pacific Standard Time",PreferredSupportLanguage:"en-us",AdditionalEmailAddresses:[a@x b@y]}, Require24X7Response=true, ProblemStartTime=2026-11-20T00:00:00Z
func TestTicketsCreateTechnicalResourceMismatch(t *testing.T) // technical svc, resource in other sub -> error "… does not match with sub1", no CheckName call
func TestTicketsCreateQuotaFlags(t *testing.T)   // --quota-change-requests '[{"payload":"{}","region":"eastus"}]' -> QuotaTicketDetails.QuotaChangeRequests[0].Region=="eastus"
func TestTicketsListDefaultFilter(t *testing.T)  // now=2026-11-24 -> fake records ListTickets filter "CreatedDate ge 2026-11-17"; with --filter "Status eq 'Open'" -> passed through; --max-items 5 with no --filter -> filter ""; fake 3 tickets + --max-items 2 -> 2 printed
func TestTicketsUpdateOnlyChanged(t *testing.T)  // --status closed -> UpdateSupportTicket{Status:closed}, ContactDetails nil, Severity nil
func TestParseStartTime(t *testing.T)            // three formats OK; "yesterday" -> error
func TestParseSecondaryConsent(t *testing.T)     // `[{"type":"x","user-consent":"Yes"}]` -> 1 item; `[{"user-consent":"maybe"}]` -> error
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run 'TestTickets|TestParse|TestFlagParity' -v` and confirm it FAILS.
- [ ] **Step 3: Implement the commands in `tickets.go`**
- [ ] **Step 4: Run the same command and confirm it PASSES. Then run `make build && ./bin/az/az support in-subscription tickets create --help` and check the help lists every flag in the table.**
- [ ] **Step 5: Commit** `feat(support): add support tickets commands`

---

### Task 6: communication, chat-transcript, file, file-workspace commands

**Files:**
- Create: `internal/support/communication.go`
- Modify: `internal/support/files.go`, `internal/support/commands_test.go`
- Test: `internal/support/communication_test.go`, `internal/support/files_test.go`

**Interfaces:**
- Consumes: `commandClientFn`, `readUpload`, `uploadFile`, `typeFileWorkspaces`, `workspaceNamePattern`.
- Produces: `newCommunicationCmd(tenantLevel bool)`, `newChatTranscriptCmd(tenantLevel bool)`, `newFileCmd(tenantLevel bool)`, `newFileWorkspaceCmd(tenantLevel bool)`. All are wired into `newScopeGroup`.

Parity rows (`<S>` means both scopes):

| Path | Flags |
|---|---|
| `<S> communication create` | `*ticket-name *communication-name *communication-subject *communication-body communication-sender` |
| `<S> communication list` | `*ticket-name filter max-items` |
| `<S> communication show` | `*ticket-name *communication-name` |
| `<S> chat-transcript list` | `*ticket-name max-items` |
| `<S> chat-transcript show` | `*ticket-name *chat-transcript-name` |
| `<S> file upload` | `*file-workspace-name *file-path` |
| `<S> file list` | `*file-workspace-name max-items` |
| `<S> file show` | `*file-workspace-name *file-name` |
| `<S> file-workspace create` | `*file-workspace-name` |
| `<S> file-workspace show` | `*file-workspace-name` |

Behaviour:
- The list commands take `--max-items` with the Task 5 rule (truncate after paging).
- `communication create`: `CheckCommunicationName`, then `CreateCommunication`, then print.
- `file-workspace create`: validate the name against `workspaceNamePattern`, then `CheckName(name, typeFileWorkspaces)`, then create and print.
- `file upload`: `readUpload`, then `uploadFile`. On success, print `File '<name>' has been successfully uploaded.` to stdout, which is the exact Python text.

These are not exposed: `communication update`, `file update`, `file create`, and the raw chunk upload.

- [ ] **Step 1: Write the failing tests**

```go
func TestCommunicationCreateChecksName(t *testing.T) // calls == [CheckCommunicationName:t1/c1, CreateCommunication:t1/c1]; stored Subject/Body/Sender
func TestCommunicationCreateNameTaken(t *testing.T)  // takenNames{"c1"} -> error, no CreateCommunication
func TestFileWorkspaceCreateBadName(t *testing.T)    // "a/b" -> error, no API calls
func TestFileUploadPrintsPythonMessage(t *testing.T) // stdout == "File 'notes.txt' has been successfully uploaded.\n"
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run 'TestCommunication|TestFileWorkspace|TestFileUpload|TestFlagParity' -v` and confirm it FAILS.
- [ ] **Step 3: Implement the commands**
- [ ] **Step 4: Run the same command and confirm it PASSES. Then run `make build`.**
- [ ] **Step 5: Commit** `feat(support): add communication, chat-transcript and file commands`

---

### Task 7: Search ranking and the service/problem catalog

**Files:**
- Create: `internal/support/search.go`
- Modify: `internal/support/services.go`
- Test: `internal/support/search_test.go`, `internal/support/services_test.go`

**Interfaces:**
- Consumes: `Client.ListServices` and `ListProblemClassifications`, `isQuotaService`.
- Produces:

```go
func rank(query string, items []string) []int // indices of matching items, best first
func queryFromTitle(title string) string
type catalogRow struct {
	ServiceName, ServiceDisplay string // ServiceName = service GUID (Service.Name)
	ProblemID, ProblemDisplay string   // ProblemID = full ProblemClassification.ID
	ResourceTypes []string
}
func (r catalogRow) text() string // ServiceDisplay + " › " + ProblemDisplay
func loadCatalog(ctx context.Context, c Client, workers int) ([]catalogRow, error)
```

The ranking algorithm is the one non-obvious part. Rows that match all words come first because they have the highest count.

```go
// words: unique strings.Fields(strings.ToLower(query)); empty -> all indices in order.
// score(item) = number of words w with strings.Contains(strings.ToLower(item), w).
// keep score > 0; sort.SliceStable by score desc (ties keep input order).
```

`queryFromTitle` works as follows:
1. Lower-case the title.
2. Split on runs that are not letters or digits (`unicode.IsLetter`, `unicode.IsDigit`).
3. Drop words shorter than 3 runes, and words in this stop list: `the and for with not from this that are was have has does when our you your can cannot unable after before into`.
4. Drop duplicates, keeping the first occurrence.
5. Join the rest with a space.

`loadCatalog`:
- Call `ListServices` first. If it fails, return `nil, err`.
- Then fetch problem classifications per service, with at most `workers` running at once (a buffered-channel semaphore with a `sync.WaitGroup`).
- Rows are sorted by `ServiceDisplay`, then `ProblemDisplay`.
- If some services fail, still return the rows from the others, along with `fmt.Errorf("could not load problem types for %d service(s): %w", n, firstErr)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestRank(t *testing.T) {
	items := []string{"Virtual Machine › Cannot connect", "Storage › Cannot connect to blob", "Virtual Machine › Performance"}
	require.Equal(t, []int{0, 1, 2}, rank("", items))
	require.Equal(t, []int{0, 2}, rank("virtual", items))
	require.Equal(t, []int{0, 1, 2}, rank("virtual connect", items)) // 0 has both, 1 and 2 one each, input order kept
	require.Equal(t, []int{1}, rank("BLOB", items))
	require.Empty(t, rank("kubernetes", items))
}
func TestQueryFromTitle(t *testing.T) // "Cannot connect to my VM after the reboot!!" -> "connect reboot"
func TestLoadCatalog(t *testing.T)            // 2 services x 2 problems -> 4 rows sorted; row.ProblemID == full id
func TestLoadCatalogPartialFailure(t *testing.T) // svc B problems error -> rows of A returned, err contains "1 service(s)"
func TestLoadCatalogServicesError(t *testing.T)  // ListServices error -> nil rows, err
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run 'TestRank|TestQueryFromTitle|TestLoadCatalog' -v` and confirm it FAILS.
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): add local service and problem search`

---

### Task 8: HTML to text

**Files:**
- Create: `internal/support/htmltext.go`
- Test: `internal/support/htmltext_test.go`

**Interfaces:**
- Produces: `func htmlToText(s string) string`

Algorithm (stdlib only: `regexp` and `html`; RE2 has no backreferences):
1. Remove `(?is)<script[^>]*>.*?</script>` and `(?is)<style[^>]*>.*?</style>`.
2. Replace `(?i)<br\s*/?>` with `\n`, `(?i)</(p|div|li|tr|h[1-6])>` with `\n`, and `(?i)<li[^>]*>` with `- `.
3. Remove `<[^>]*>`.
4. Run `html.UnescapeString`, then replace ` ` with a space.
5. Normalise `\r\n` to `\n`, trim trailing spaces on each line, collapse 3 or more newlines to 2, and `TrimSpace`.

- [ ] **Step 1: Write the failing test**

```go
func TestHTMLToText(t *testing.T) {
	cases := map[string]string{
		"plain text":                                  "plain text",
		"<p>Hello&nbsp;there</p><p>Second</p>":        "Hello there\nSecond",
		"Line1<br>Line2<BR/>Line3":                    "Line1\nLine2\nLine3",
		"<ul><li>a</li><li>b</li></ul>":               "- a\n- b",
		"<style>p{}</style><div>x &amp; y &lt;z&gt;</div>": "x & y <z>",
		"<p>a</p>\n\n\n\n<p>b</p>":                    "a\n\nb",
	}
	for in, want := range cases { require.Equal(t, want, htmlToText(in), in) }
}
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run TestHTMLToText -v` and confirm it FAILS.
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): convert HTML message bodies to text`

---

### Task 9: Contact defaults (time zone, language, country, Graph /me)

**Files:**
- Create: `internal/support/tzdata.go`, `internal/support/contact.go`, and these files under `internal/support/cldr/`:
  - `windowsZones.xml`
  - `timezone.xml`
  - `LICENSE`
- Modify: `go.mod` (`golang.org/x/text` moves from indirect to direct)
- Test: `internal/support/tzdata_test.go`, `internal/support/contact_test.go`

**Interfaces:**
- Produces:

```go
func windowsZone(iana string) (string, bool)
func windowsZoneNames() []string // distinct `other` values, sorted (142 in release-49)
func ianaFromTZ(tzEnv, localtimeLink string) string
func localIANAZone() string // ianaFromTZ(os.Getenv("TZ"), readlink("/etc/localtime"))
var supportLanguages = []string{"en-us", "zh-hans", "es-es", "fr-fr", "ja-jp", "ko-kr", "ru-ru", "pt-br", "it-it", "zh-tw", "de-de"}
func supportLanguage(getenv func(string) string) string
func countryAlpha3(code string) (string, bool)
func countryOptions() []string // "Canada (CAN)", sorted by name
type me struct{ GivenName, Surname, Mail, UserPrincipalName, UsageLocation string } // json tags givenName,surname,mail,userPrincipalName,usageLocation
var graphBaseURL = "https://graph.microsoft.com" // test seam
func fetchMe(ctx context.Context, cred azcore.TokenCredential) (me, error)
type contactDefaults struct{ FirstName, LastName, Email, Country, TimeZone, Language string }
func contactFromMe(m me, getenv func(string) string, ianaZone string) contactDefaults
```

Data:
- Download the three files from CLDR tag `release-49`:
  - `https://raw.githubusercontent.com/unicode-org/cldr/release-49/common/supplemental/windowsZones.xml`
  - `.../release-49/common/bcp47/timezone.xml`
  - `.../release-49/LICENSE`
- Embed the two XML files with `//go:embed cldr/windowsZones.xml cldr/timezone.xml`.
- Parse them once with `sync.OnceValue` and `encoding/xml`. From `windowsZones.xml`, every `mapZone` `type` token (space-separated) maps to `other`. From `timezone.xml`, each `<type alias="A B C">` defines one alias group. If any member of a group maps to a Windows zone, every member maps to it. This is what makes `Asia/Kolkata` resolve, because the group contains `Asia/Calcutta`.

`ianaFromTZ`:
1. Use `tzEnv` without its leading `:`.
2. If the result (or else `localtimeLink`) contains `zoneinfo/`, take the part after the last `zoneinfo/`.
3. Return `""` for `"Local"`, for `""`, and on Windows (where Go reports `Local`, so the picker is shown).

`supportLanguage`:
- Read the first non-empty value of `LC_ALL`, `LC_MESSAGES` and `LANG`.
- Strip `.charset` and `@modifier`, then split it into a language and a region on `_` or `-`.
- Map it as follows:

| Locale | Code |
|---|---|
| `zh` with region `TW`, `HK` or `MO`, or script `Hant` | `zh-tw` |
| other `zh` | `zh-hans` |
| `es` | `es-es` |
| `fr` | `fr-fr` |
| `ja` | `ja-jp` |
| `ko` | `ko-kr` |
| `ru` | `ru-ru` |
| `pt` | `pt-br` |
| `it` | `it-it` |
| `de` | `de-de` |
| anything else, including `C` and `POSIX` | `en-us` |

`countryAlpha3` uses `language.ParseRegion`. It accepts alpha-2 or alpha-3 in any case, and returns `r.ISO3()` only when all of these hold (otherwise `"", false`):
- `r.IsCountry()` and `len(r.ISO3()) == 3`;
- `r.ISO3() != "ZZZ"` (x/text's placeholder for `UK`, `EZ`, `UN`, `EA`, `IC`, `CQ`);
- `r.Canonicalize() == r` (drops deprecated codes such as `DD`, `FX`, `TP`, `ZR`, `BU`, `FQ`);
- `display.English.Regions().Name(r) != ""`.

`countryOptions` tries `AA` through `ZZ` with the same rule and names each country with `display.English.Regions().Name(r)`. With x/text v0.37.0 this gives 254 entries with distinct alpha-3 codes.

`fetchMe`:
- Sends `GET {graphBaseURL}/v1.0/me?$select=givenName,surname,mail,userPrincipalName,usageLocation` with a bearer token for `https://graph.microsoft.com/.default`, using the pattern at `internal/keyvault/principal.go:55`.
- A non-200 response returns an error that includes the status.

`contactFromMe`:
- `Email` is `Mail`. When `Mail` is empty, fall back to the UPN, unless the UPN contains `#EXT#`; then leave it empty.
- `Country` is `countryAlpha3(UsageLocation)`, or `""`.
- `TimeZone` is `windowsZone(ianaZone)`, or `""`.

- [ ] **Step 1: Download the CLDR files.** `golang.org/x/text v0.37.0` is already in go.mod (as indirect), so no `go get`.

- [ ] **Step 2: Write the failing tests**

```go
func TestWindowsZone(t *testing.T)        // "America/Los_Angeles"->"Pacific Standard Time"; "America/Vancouver"->"British Columbia Standard Time" (CLDR 49 split it from Pacific); "Etc/UTC"->"UTC"; "Mars/Base"->"",false
func TestWindowsZoneAliases(t *testing.T) // "Asia/Kolkata"->"India Standard Time"; "Europe/Kyiv"->"FLE Standard Time"; "UTC"->"UTC"
func TestWindowsZoneNames(t *testing.T)   // contains "Pacific Standard Time", sorted, no duplicates
func TestIANAFromTZ(t *testing.T)         // (":Europe/Paris","")->"Europe/Paris"; ("","/var/db/timezone/zoneinfo/America/Vancouver")->"America/Vancouver"; ("","/usr/share/zoneinfo/Asia/Kolkata")->"Asia/Kolkata"; ("","")->""
func TestSupportLanguage(t *testing.T)    // LANG=de_DE.UTF-8->de-de; LC_ALL=zh_TW.UTF-8 beats LANG=fr_FR->zh-tw; LANG=zh_CN->zh-hans; LANG=pt_PT->pt-br; LANG=C->en-us; none->en-us
func TestCountryAlpha3(t *testing.T)      // "US"->"USA"; "gb"->"GBR"; "CAN"->"CAN"; "XX"->"",false; "UK"->"",false; "EZ"->"",false; "DD"->"",false
func TestCountryOptions(t *testing.T)     // no option contains "(ZZZ)"; no duplicate alpha-3 codes; contains "Canada (CAN)"
func TestContactFromMe(t *testing.T)      // full -> all fields; Mail=""+UPN "a@b.com" -> Email "a@b.com"; Mail=""+UPN "a_b.com#EXT#@t.onmicrosoft.com" -> Email ""; UsageLocation "" -> Country ""
func TestFetchMe(t *testing.T)            // httptest server: asserts path "/v1.0/me", $select, "Authorization: Bearer tok"; decodes fields
func TestFetchMeError(t *testing.T)       // 403 -> error containing "403"
```

- [ ] **Step 3: Run** `go test ./internal/support/ -run 'TestWindowsZone|TestIANAFromTZ|TestSupportLanguage|TestCountry|TestContactFromMe|TestFetchMe' -v` and confirm it FAILS.
- [ ] **Step 4: Implement `tzdata.go` and `contact.go`, then run `go mod tidy`.** Expected: go.mod lists `golang.org/x/text` in the direct block.
- [ ] **Step 5: Run the same command and confirm it PASSES.**
- [ ] **Step 6: Commit** `feat(support): derive contact defaults from Graph, locale and time zone`

---

### Task 10: Resource listing for technical tickets

**Files:**
- Create: `internal/support/resources.go`
- Test: `internal/support/resources_test.go`

**Interfaces:**
- Produces:

```go
type resourceRef struct{ ID, Name, Type string }
func resourceTypeFilter(types []string) string // "resourceType eq 'A' or resourceType eq 'B'"; single quotes in values doubled
func listResources(ctx context.Context, s Scope, types []string) ([]resourceRef, error) // armresources.NewClient(s.SubscriptionID, cred, nil).NewListPager(&armresources.ClientListOptions{Filter: &f}); all pages; sorted by Name
```

- [ ] **Step 1: Write the failing test.** `TestResourceTypeFilter`: `["Microsoft.Compute/virtualMachines"]` gives `"resourceType eq 'Microsoft.Compute/virtualMachines'"`; two types are joined with `" or "`.
- [ ] **Step 2: Run** `go test ./internal/support/ -run TestResourceTypeFilter -v` and confirm it FAILS.
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): list subscription resources by type`

---

### Task 11: TUI dependencies, app shell, picker, scope switcher, launch

**Files:**
- Modify: `go.mod`, `go.sum`, `internal/support/commands.go` (replace the `runTUI` stub)
- Modify: `pkg/azure/discovery.go` (export `NeedsInteractiveAuth`)
- Create: `internal/support/tui_app.go`, `internal/support/tui_picker.go`, `internal/support/tui_scope.go`
- Modify: `internal/support/fake_test.go` (add `fakeDeps`, `drive`, `keys`)
- Test: `internal/support/tui_scope_test.go`, `internal/support/tui_picker_test.go`

**Interfaces:**
- Consumes:
  - `Scope`, `NewClient`, `rank`.
  - `config.Load()`.
  - `azure.DiscoverAllSubscriptionsFromCache(ctx) ([]azure.TenantInfo, error)` (`pkg/azure/discovery.go:214`).
  - `azure.AuthenticateForTenant(ctx, tenantID) (azcore.TokenCredential, error)` (`:347`).
  - `azure.DiscoverTenantSubscriptions(ctx, tenantID, cred) ([]config.Subscription, error)` (`:401`).
  - `needsInteractiveAuth` (`pkg/azure/discovery.go:332`), exported in this task as `azure.NeedsInteractiveAuth(err error) bool`. The unexported name stays as a one-line call to it, so `discovery_test.go` still passes. It covers the AADSTS codes and broker `interactionRequired` errors (`pkg/azure/broker.go:182`).
- Produces:

```go
type deps struct {
	newClient     func(Scope) (Client, error)
	loadProfile   func() (*config.Profile, error)
	discover      func(ctx context.Context) ([]azure.TenantInfo, error)
	signIn        func(ctx context.Context, tenantID string) ([]config.Subscription, error) // AuthenticateForTenant + DiscoverTenantSubscriptions
	listResources func(ctx context.Context, s Scope, types []string) ([]resourceRef, error)
	fetchMe       func(ctx context.Context, s Scope) (me, error) // credentialFor(s.TenantID) + fetchMe
	localZone     func() string
	getenv        func(string) string
}
func defaultDeps() deps

type screen interface {
	init(a *app) tea.Cmd // starts the screen's load; called by push, replace and setScope
	update(a *app, msg tea.Msg) tea.Cmd
	view(a *app) string
}
type app struct {
	ctx      context.Context
	d        deps
	scope    Scope
	client   Client
	gen      int          // incremented on every scope change; async msgs carry it
	stack    []screen
	catalog  []catalogRow // cached for the process
	catErr   error
	width, height int
	quitting bool
}
func newApp(ctx context.Context, d deps, s Scope) (*app, error) // builds client; stack = [newListScreen()]
func (a *app) Init() tea.Cmd
func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) // ctrl+c quits; WindowSizeMsg stored; else top().update
func (a *app) View() tea.View                          // header (a.scope.Label()) + top().view + footer; v.AltScreen = true
func (a *app) push(s screen) tea.Cmd    // append; return s.init(a)
func (a *app) pop()
func (a *app) replace(s screen) tea.Cmd // pop, then push
func (a *app) setScope(s Scope) tea.Cmd // newClient; gen++; stack = [newListScreen()]; returns a.stack[0].init(a)
type reauthDoneMsg struct{ err error } // the ctrl+l tea.Exec callback returns it; nil err -> setScope(a.scope) again
func startScope(cmd *cobra.Command, p *config.Profile) (Scope, error) // --tenant T -> {TenantID:T}; --subscription -> that sub; else default sub (IsDefault, else [0])
func runTUI(cmd *cobra.Command) error // startScope -> newApp(cmd.Context(), defaultDeps(), s) -> tea.NewProgram(a, tea.WithContext(ctx)).Run()

type picker struct { input textinput.Model; items []string; shown []int; cursor int }
func newPicker(items []string, query string) picker
func (p *picker) setItems(items []string)
func (p *picker) update(msg tea.Msg) (picked int, done bool, cmd tea.Cmd) // up/down move; enter -> (shown[cursor], true); typing refilters via rank over the item text
func (p picker) view(height int) string

type scopeScreen struct { /* picker rows; rowScopes []Scope (zero Scope for headings/"Other tenant…") */ }
func newScopeScreen(p *config.Profile) *scopeScreen
type tenantsLoadedMsg struct{ gen int; tenants []azure.TenantInfo; err error }
type tenantSignedInMsg struct{ tenantID string; subs []config.Subscription; err error }
```

Test helpers in `fake_test.go`:

```go
func keys(s ...string) []tea.Msg // "enter","esc","tab","shift+tab","up","down","space","ctrl+s","ctrl+r","ctrl+n" -> KeyPressMsg{Code/Mod}; any other string -> one KeyPressMsg{Code:r, Text:string(r)} per rune
func drive(t *testing.T, a *app, msgs ...tea.Msg) // for each msg: Update; run returned Cmd synchronously, expanding tea.BatchMsg, dropping nil, spinner.TickMsg and unknown internal msgs, depth cap 50. Relies on cursor blink being off (Global Constraints); otherwise every Cmd waits 530 ms
func fakeDeps(fc *fakeClient) deps
```

Scope rows come from `loadProfile().Subscriptions`, grouped by `TenantID`, with tenants in first-seen order. Each group is:
- a heading `Tenant <id>`. Its `rowScopes` entry is the zero `Scope`, and `enter` on it does nothing;
- `  Tenant-level tickets (no subscription) · <tenant id>`. The tenant ID is in the text so typing it filters to this row, and filtered rows from different tenants stay distinguishable;
- `  <sub name> (<sub id>)` for each subscription.

The last row is `Other tenant…`. Picking it:
1. Runs `d.discover` with a spinner.
2. Replaces the rows with each tenant's `DisplayName (TenantID)`, including a tenant-level row and its subscriptions. Tenants with `NeedsMFA` get a single row: `DisplayName (TenantID) — sign in`.
3. Picking a sign-in row returns `tea.Exec(signInCommand{...}, func(err error) tea.Msg {...})`. `signInCommand` implements `tea.ExecCommand`: `Run` calls `d.signIn`, and the setters are no-ops. The program then releases the terminal for the browser sign-in.
4. `tenantSignedInMsg` replaces the rows with that tenant's level row and its subscriptions.

Picking a scope row calls `a.setScope`. `esc` pops. Errors are shown in the screen as `Error: <err>`.

Credential errors:
- When `setScope` or any later client call fails with an error for which `azure.NeedsInteractiveAuth(err)` is true, show the error and the hint `press ctrl+l to sign in to this tenant`.
- `ctrl+l` runs the same `tea.Exec` sign-in for `a.scope.TenantID`. Its callback returns `reauthDoneMsg{err}`; with a nil error the app calls `setScope(a.scope)` again.

- [ ] **Step 1: Add the dependencies**

Run: `go get charm.land/bubbletea/v2@v2.1.2 charm.land/bubbles/v2@v2.2.2 charm.land/lipgloss/v2@v2.1.1`
Expected: go.mod lists all three, and the `go` directive is now `1.26.0`.

- [ ] **Step 2: Write the failing tests**

```go
func TestPickerFilters(t *testing.T)        // items a/"Pacific Standard Time"/"Mountain Standard Time"; type "pacific" -> shown == [1]; enter -> picked 1
func TestStartScope(t *testing.T)           // profile 2 subs (default s2/t2): no flags -> {t2,s2,name}; --tenant t9 -> {TenantID:"t9"}; --subscription s1 -> {t1,s1}
func TestScopeScreenRows(t *testing.T)      // 2 tenants x 1 sub: view contains "Tenant t1", "Tenant-level tickets (no subscription) · t1", "Sub One (s1)", "Other tenant…"; enter on the "Tenant t1" heading leaves a.scope unchanged
func TestScopeSwitchToTenantLevel(t *testing.T) // drive: "t", type "t2", select tenant-level row, enter -> a.scope == Scope{TenantID:"t2"}; a.gen incremented; fakeDeps.newClient called with it
func TestScopeOtherTenantDiscovery(t *testing.T) // pick "Other tenant…" -> discover called; rows include "Fabrikam (t3)" and "Contoso (t4) — sign in"
func TestScopeSignedInRows(t *testing.T)    // Update(tenantSignedInMsg{"t4", subs:[s9]}) -> rows include "Tenant-level tickets (no subscription) · t4" and "s9"
func TestScopeDoesNotWriteProfile(t *testing.T) // after switching, profile file mtime unchanged (t.Setenv HOME to temp; config.Save never called)
func TestCredentialErrorOffersSignIn(t *testing.T) // fakeDeps.newClient returns errors.New("AADSTS50076: MFA required") -> view contains "press ctrl+l to sign in to this tenant"; drive reauthDoneMsg{nil} -> newClient called again with a.scope
```

- [ ] **Step 3: Run** `go test ./internal/support/ -run 'TestPicker|TestStartScope|TestScope|TestCredentialError' -v` and confirm it FAILS.
- [ ] **Step 4: Export `azure.NeedsInteractiveAuth`, implement `tui_app.go`, `tui_picker.go` and `tui_scope.go`, and replace the `runTUI` stub**

`newListScreen` is a placeholder until Task 12: `init` returns nil, it renders `"tickets"`, and its `update` handles only `t` (push `newScopeScreen(profile)`) and `q` (quit), so `TestScopeSwitchToTenantLevel` can drive `t`.

- [ ] **Step 5: Run the same command and `go test ./pkg/azure/ -run TestNeedsInteractiveAuth`, and confirm both PASS. Then run `make build`; `./bin/az/az support` on a TTY opens the TUI, and `t` shows the profile's tenants. Run `./bin/az/az support < /dev/null` and check that it prints help.**
- [ ] **Step 6: Commit** `feat(support): add support TUI shell and scope switcher`

---

### Task 12: Ticket list screen

**Files:**
- Create: `internal/support/tui_list.go`
- Test: `internal/support/tui_list_test.go`

**Interfaces:**
- Consumes: `app`, `picker`, `rank`, `Client.ListTickets`.
- Produces:

```go
const openFilter = "Status eq 'Open'"
func recentFilter(t time.Time) string // "CreatedDate ge " + t.AddDate(0,0,-30).Format("2006-01-02")
type listScreen struct{ all bool; tickets []*armsupport.TicketDetails; loading bool; err error; search textinput.Model; searching bool; cursor int }
func newListScreen() *listScreen
type ticketsLoadedMsg struct{ gen int; all bool; tickets []*armsupport.TicketDetails; err error }
func (l *listScreen) init(a *app) tea.Cmd // ListTickets for the current filter; f and ctrl+r call it too
func ticketRowText(t *armsupport.TicketDetails) string // "2026-11-20 14:03  moderate  Open  <title>  <supportTicketId>"
```

Behaviour:
- The list loads in `init`, which runs on push and after a scope change.
- Rows are sorted by `CreatedDate`, newest first.
- `f` toggles between `openFilter` and `recentFilter(now())`, and the header shows `open` or `all, last 30 days`.
- `/` focuses the search; typing filters the rows with `rank` over `ticketRowText`, and `esc` clears it.
- `enter` pushes `newDetailScreen(ticketName)`, which comes in Task 13. Use a placeholder until then.
- `t` pushes the scope screen, `ctrl+r` reloads and `q` quits. `n` is not bound until Task 15 wires it.
- A `ticketsLoadedMsg` whose `gen != a.gen` or whose `all != l.all` is dropped.
- Errors are rendered as `Error: <err>`.

- [ ] **Step 1: Write the failing tests**

```go
func TestListLoadsOpenTicketsNewestFirst(t *testing.T) // fake 2 tickets (older first) -> view lists newer first; fake ListTickets filter == "Status eq 'Open'"
func TestListToggleFilter(t *testing.T)                // drive "f" -> last filter == "CreatedDate ge 2026-10-25" (now 2026-11-24)
func TestListSearch(t *testing.T)                      // "/" then "vm" -> only matching title visible
func TestListShowsError(t *testing.T)                  // ListTickets err "boom" -> view contains "Error: boom"
func TestListIgnoresStaleScopeResults(t *testing.T)    // a.gen=2; Update(ticketsLoadedMsg{gen:1, tickets:[x]}) -> x not shown
func TestListIgnoresStaleFilterResults(t *testing.T)   // l.all=true; Update(ticketsLoadedMsg{gen:a.gen, all:false, tickets:[x]}) -> x not shown
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run TestList -v` and confirm it FAILS.
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): add TUI ticket list`

---

### Task 13: Ticket detail screen (thread, status, severity, attach)

**Files:**
- Create: `internal/support/tui_detail.go`
- Test: `internal/support/tui_detail_test.go`

**Interfaces:**
- Consumes: `htmlToText`, `readUpload`, `uploadFile`, `needsPhone`, `picker`, `Client.GetTicket`, `ListCommunications`, `UpdateTicket`.
- Produces:

```go
type detailScreen struct{ name string; ticket *armsupport.TicketDetails; comms []*armsupport.CommunicationDetails; vp viewport.Model; mode detailMode; input textinput.Model; sev picker; status string; err error }
type detailMode int // modeView, modeConfirmStatus, modeSeverity, modePhone, modeAttach
func newDetailScreen(name string) *detailScreen
type ticketLoadedMsg struct{ gen int; name string; ticket *armsupport.TicketDetails; comms []*armsupport.CommunicationDetails; err error } // dropped when gen != a.gen or name != d.name
func renderThread(t armsupport.TicketDetails, comms []*armsupport.CommunicationDetails) string
const errNoWorkspace = "this ticket has no file workspace"
```

`renderThread` builds, in order:
1. A header with the title, `SupportTicketID`, `Status`, `Severity`, `ServiceDisplayName › ProblemClassificationDisplayName`, the created and modified times, and `SupportPlanDisplayName`.
2. `htmlToText(Description)`.
3. The communications, sorted by `CreatedDate` ascending. Each is shown as `── <from Microsoft|from you> · <Sender> · <time> ──`, then `Subject`, then `htmlToText(Body)`. `inbound` means from Microsoft.

`init` loads with `GetTicket` and `ListCommunications(name, "")` in one Cmd. Keys:
- `r` is not bound until Task 14 wires it to push `newReplyScreen(ticket)`.
- `a`: if `FileWorkspaceName` is empty, set `err = errNoWorkspace`. Otherwise open a path input; on `enter`, run `readUpload` and then `uploadFile`, and set `status = "Uploaded <name>"`.
- `s` asks `Close this ticket? (y/n)` (or `Reopen`). `y` sends `UpdateTicket{Status: closed|open}` and reloads. Status is compared case-insensitively.
- `v` opens the severity picker. If the new severity needs a phone number and `ContactDetails.PhoneNumber` is empty, ask for one, then send `UpdateTicket{Severity, ContactDetails: &UpdateContactProfile{PhoneNumber}}`.
- `esc` pops (or cancels the current mode), and `ctrl+r` reloads.

- [ ] **Step 1: Write the failing tests**

```go
func TestRenderThreadOrderAndHTML(t *testing.T) // comms out of order, one inbound HTML "<p>Hi&amp;bye</p>" -> "from Microsoft" block before later "from you" block; contains "Hi&bye"
func TestDetailCloseTicket(t *testing.T)        // status "Open"; drive "s","y" -> fake UpdateTicket{Status: closed}; then reload called
func TestDetailReopenTicket(t *testing.T)       // status "Closed"; drive "s","y" -> Status open
func TestDetailSeverityNeedsPhone(t *testing.T) // no phone; drive "v", pick critical, enter, type "+15550100", enter -> Update{Severity:critical, ContactDetails.PhoneNumber:"+15550100"}
func TestDetailAttachNoWorkspace(t *testing.T)  // FileWorkspaceName nil; drive "a" -> view contains "this ticket has no file workspace"; no CreateFile call
func TestDetailAttachUploads(t *testing.T)      // workspace "ws1"; drive "a", type temp path, enter -> CreateFile:ws1/<name>; view contains "Uploaded"
func TestDetailIgnoresOtherTicketLoad(t *testing.T) // d.name="Y"; Update(ticketLoadedMsg{gen:a.gen, name:"X", ticket:X}) -> X title not shown
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run 'TestRenderThread|TestDetail' -v` and confirm it FAILS.
- [ ] **Step 3: Implement, and replace the Task 12 `enter` placeholder**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): add TUI ticket detail with status, severity and attach`

---

### Task 14: Reply editor

**Files:**
- Create: `internal/support/tui_reply.go`
- Test: `internal/support/tui_reply_test.go`

**Interfaces:**
- Consumes: `newName`, `readUpload`, `uploadFile`, `Client.CheckCommunicationName`, `CreateCommunication`.
- Produces:

```go
type replyScreen struct{ ticket armsupport.TicketDetails; subject textinput.Model; body textarea.Model; attach textinput.Model; focus int; sending bool; err error }
func newReplyScreen(t armsupport.TicketDetails) *replyScreen // subject = *t.Properties.Title
type replySentMsg struct{ gen int; err error }
```

`tab` and `shift+tab` move between subject, body and attachment path. `ctrl+s` sends, in this order:
1. Body empty: show the error `Message body is required`.
2. Attachment path given but no workspace: show `errNoWorkspace`.
3. Attachment path given: `readUpload`, which runs before any API call.
4. `name := newName()`.
5. `CheckCommunicationName(ticket, name)`.
6. Upload the attachment, if any.
7. `CreateCommunication(ticket, name, CommunicationDetails{Properties: {Subject, Body}})`.

On success it calls `a.pop()` and returns the detail screen's `init(a)`, so the thread reloads. On error it stays, shows the error, and keeps the text. `esc` pops without sending.

- [ ] **Step 1: Write the failing tests**

```go
func TestReplySends(t *testing.T)          // newName stub "c-1"; drive tab, type "Thanks", ctrl+s -> calls [CheckCommunicationName:T/c-1, CreateCommunication:T/c-1]; stored Subject == ticket title, Body == "Thanks"; top screen is detail
func TestReplyEmptyBody(t *testing.T)      // ctrl+s -> view contains "Message body is required"; no calls
func TestReplyErrorKeepsText(t *testing.T) // CreateCommunication err "denied" -> view contains "Error: denied" and body still "Thanks"
func TestReplyAttachBadFile(t *testing.T)  // attach "x.exe" -> "The file extension is not supported."; no API calls
func TestReplyAttachNoWorkspace(t *testing.T) // FileWorkspaceName nil; body "x", attach path set, ctrl+s -> view contains "this ticket has no file workspace"; no CheckCommunicationName or CreateFile calls
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run TestReply -v` and confirm it FAILS.
- [ ] **Step 3: Implement, and wire `r` in the detail screen**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): add TUI reply editor`

---

### Task 15: New-ticket flow, steps 1–3 (describe, service search, resource)

**Files:**
- Create: `internal/support/tui_create.go`
- Test: `internal/support/tui_create_test.go`

**Interfaces:**
- Consumes: `loadCatalog`, `catalogRow`, `rank`, `queryFromTitle`, `isTechnicalService`, `isQuotaService`, `parseProblemClassificationID`, `validateTechnicalResource`, `deps.listResources`, `picker`.
- Produces (Task 16 extends):

```go
type createStep int // stepDescribe, stepService, stepResource, stepSeverity, stepContact, stepAttach, stepReview
type createScreen struct {
	step        createStep
	title       textinput.Model
	desc        textarea.Model
	start       textinput.Model // empty = now
	svc         picker
	row         *catalogRow
	resources   []resourceRef
	resPick     picker
	resourceID  string
	resInput    textinput.Model
	// Task 16 fields: severity, require24x7, consent, contact inputs, attach, err, submitting
}
func newCreateScreen() *createScreen
func (c *createScreen) init(a *app) tea.Cmd // starts loadCatalog(a.ctx, a.client, 8) unless a.catalog is cached
type catalogLoadedMsg struct{ rows []catalogRow; err error }
const quotaHint = "Quota requests are not filed here. Use `az quota` or `az support in-subscription tickets create --quota-change-*`."
func (c *createScreen) needsResourceStep(a *app) bool // c.row != nil && isTechnicalService(c.row.ServiceName) && !a.scope.TenantLevel() && len(c.row.ResourceTypes) > 0
```

Keys used in every step:
- `tab` and `shift+tab` move between fields.
- `ctrl+n` goes to the next step. Before moving on, it validates the current step.
- `esc` goes back one step, keeping all values. From step 1 it pops the screen.
- `enter` in a list picks and moves to the next step, the same as `ctrl+n`. From a single-line field it also moves to the next step. (The contact step's country, time zone and language fields are the exception: there `enter` opens their picker.)

Steps:
- **Describe:** title and description are required (`Title is required`, `Description is required`). The start time is parsed with `parseStartTime` when it is not empty.
- **Service:**
  - The picker items are `row.text()`. Its query starts as `queryFromTitle(title)`, set once on first entry.
  - While `catalogLoadedMsg` is pending, show a spinner and still accept typing.
  - `catErr` is shown above the list.
  - Picking a quota row shows `quotaHint` and stays on the step.
- **Resource:** only when `needsResourceStep`. It starts `deps.listResources(scope, row.ResourceTypes)`. The rows are `Skip`, `Enter resource ID…`, then `Name (Type)` for each resource. The ID entry is checked with `validateTechnicalResource(id, a.scope.SubscriptionID)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateDescribeRequiresTitle(t *testing.T)  // "n", ctrl+n -> view contains "Title is required"; step stays describe
func TestCreateServiceSearchPrefill(t *testing.T)   // title "Cannot connect to VM after reboot", desc, ctrl+n -> picker query == "connect reboot"; catalog rows ranked with "Virtual Machine › Cannot connect" first
func TestCreateServiceWhileLoading(t *testing.T)    // catalog msg withheld: typing updates query; view shows "Loading"
func TestCreateQuotaShowsHint(t *testing.T)         // pick quota row -> view contains quotaHint, step == stepService
func TestCreateResourceStepSkippedForBilling(t *testing.T) // billing row -> next step == stepSeverity
func TestCreateResourceStepTenantLevelSkipped(t *testing.T) // technical row, tenant-level scope -> stepSeverity
func TestCreateResourcePick(t *testing.T)           // technical row, listResources -> [vm1]; pick "vm1 (...)" -> resourceID == vm1 ID
func TestCreateEscKeepsValues(t *testing.T)         // at service step, esc -> stepDescribe, title still set
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run TestCreate -v` and confirm it FAILS.
- [ ] **Step 3: Implement, and wire `n` in the list screen to `a.push(newCreateScreen())`**
- [ ] **Step 4: Run the same command and confirm it PASSES.**
- [ ] **Step 5: Commit** `feat(support): add TUI new-ticket describe and service search`

---

### Task 16: New-ticket flow, steps 4–7 (severity, contact, attach, review/submit)

**Files:**
- Modify: `internal/support/tui_create.go`
- Test: `internal/support/tui_create_test.go`

**Interfaces:**
- Consumes:
  - `contactFromMe`, `deps.fetchMe`, `deps.localZone`, `deps.getenv`.
  - `windowsZoneNames`, `supportLanguages`, `countryOptions`, `countryAlpha3`.
  - `needsPhone`, `readUpload`, `createTicket`, `newName`, `newDetailScreen`.
- Produces:

```go
type meLoadedMsg struct{ m me; err error }
func (c *createScreen) ticketDetails(a *app) armsupport.TicketDetails // pure: builds the request from screen state
type ticketCreatedMsg struct{ gen int; name string; err error }
```

**Severity step:**
- A fixed 4-row list moved with `up`/`down`, with no filter input (not a `picker`), so `space` and `c` reach the toggles. Labels, in this order: `Minimal`, `Moderate`, `Critical`, `Highest critical impact`, mapping to the `SeverityLevel` constants. `enter` picks and moves on, like every list step.
- Help line: `Critical and highest critical impact need a support plan that allows them; the API will reject them otherwise.`
- When the severity is critical or above, `space` toggles 24x7 (default on). Below critical, `Require24X7Response` is not set.
- Advanced diagnostic consent is a Yes/No toggle on `c`, defaulting to `Yes`.

**Contact step:**
- Entering the step for the first time fires `deps.fetchMe`. `meLoadedMsg` pre-fills every empty field through `contactFromMe(m, d.getenv, d.localZone())`. On error it still pre-fills the time zone and language, and shows `Could not read your profile from Microsoft Graph: <err>` as a dim note.
- Fields:
  - first name, last name, email;
  - country: `enter` opens a picker over `countryOptions()` and stores the alpha-3 code from the parentheses;
  - time zone: `enter` opens a picker over `windowsZoneNames()`;
  - language: `enter` opens a picker over `supportLanguages`;
  - contact method: `space` toggles email/phone;
  - phone, shown only when `needsPhone(method, severity)`;
  - additional emails, comma-separated.
- `ctrl+n` checks that first name, last name, email, country, time zone and language are all set, and that the phone is set when it is needed. The error is `<Field> is required`.

**Attach step:**
- An optional path. When it is set, `readUpload` runs on `ctrl+n`, and its error stays on this step.

**Review step:**
- Renders a summary of the title, `Service › Problem`, the resource, severity, 24x7, consent, the contact lines and the attachment name.
- `enter` submits:
  1. `name := newName()`.
  2. `createTicket(ctx, client, name, c.ticketDetails(a), attachPtr)`.
  3. On success, `a.replace(newDetailScreen(name))`.
  4. On error, stay on the review step with `Error: <err>`, and `enter` can resubmit.
- A spinner shows while `submitting`, and keys are ignored during that time.

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateEndToEnd(t *testing.T) {
	// fakeDeps: fetchMe -> me{GivenName:"Ada",Surname:"Lovelace",Mail:"ada@x.com",UsageLocation:"GB"}; localZone "Europe/London"; getenv LANG=fr_FR.UTF-8; newName "11111111-1111-1111-1111-111111111111"
	// drive: n, title "Test ticket - please ignore", tab, desc "testing", ctrl+n, pick billing row, enter, (severity) enter on Minimal, (contact) ctrl+n, (attach) ctrl+n, (review) enter
	// fake stored ticket "1111…": Title, Description, Severity=minimal, AdvancedDiagnosticConsent=Yes, Require24X7Response=nil,
	//   ProblemClassificationID=billing row ProblemID, ServiceID="/providers/Microsoft.Support/services/517f2da6-…",
	//   FileWorkspaceName="1111…", ContactDetails{FirstName:"Ada",LastName:"Lovelace",PrimaryEmailAddress:"ada@x.com",Country:"GBR",
	//   PreferredTimeZone:"GMT Standard Time",PreferredSupportLanguage:"fr-fr",PreferredContactMethod:email}
	// top screen is *detailScreen with name "1111…"
}
func TestCreateCriticalNeedsPhone(t *testing.T)   // severity Critical, contact ctrl+n without phone -> "Phone number is required"; Require24X7Response=true in ticketDetails
func TestCreateGraphFailureStillPrefillsLocale(t *testing.T) // fetchMe err -> first name empty, timezone/language set, note visible
func TestCreateTechnicalResourceInRequest(t *testing.T) // resource picked -> TechnicalTicketDetails.ResourceID set
func TestCreateSubmitErrorStaysOnReview(t *testing.T) // fake CreateTicket err "plan does not allow" -> step == stepReview, view contains "Error: plan does not allow"; enter again -> second CreateTicket call
func TestCreateAttachValidatedBeforeSubmit(t *testing.T) // attach "a.bat", ctrl+n -> "The file extension is not supported.", step == stepAttach
```

- [ ] **Step 2: Run** `go test ./internal/support/ -run TestCreate -v` and confirm it FAILS.
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run `go test ./internal/support/... -v`, then `go vet ./...`, then `make build`, and confirm all PASS.**
- [ ] **Step 5: Commit** `feat(support): complete TUI new-ticket flow with contact pre-fill`

---

### Task 17: Docs

**Files:**
- Modify: `docs/user-guide.md` (Contents → "Support" group; new section `## Support tickets (az support)` before `## Terraform-compatible az version`)
- Create: `docs/design/az-support.md`
- Modify: `README.md` (capabilities list, after the PIM bullet at line 19)

Content:
- **User guide.**
  - Start the TUI with `az support`, and use `az support tui` to force it.
  - Starting scope comes from the active profile, `--subscription` or `--tenant`.
  - Keys:
    - list: `n f t / enter ctrl+r q`;
    - detail: `r a s v esc`;
    - reply: `ctrl+s`;
    - create: `tab`, `ctrl+n`, `esc`, `enter`.
  - Contact pre-fill sources.
  - Basic-plan limits.
  - Flag examples: `az support in-subscription tickets list`, `tickets show --ticket-name`, `communication create …`, `file upload --file-workspace-name --file-path`, `services problem-classifications list --service-name`.
  - Deviations from Python: JSON for list-of-object flags, StringSlice emails, the auto-created workspace, case-insensitive blocked extensions, and no `--next-token` on list commands (`--max-items` is supported).
- **Design doc.** Write it from the spec, updated to match what was built:
  - the flat package and why (the import cycle);
  - the `Client` interface;
  - the create order;
  - the CLDR data and license;
  - stale-scope generation;
  - the `tea.Exec` sign-in.
- **README bullet:** `- **Support tickets (az support)** — a full-screen TUI to file, read and reply to Azure support tickets for any subscription or tenant, with local service search and pre-filled contact details; flag commands match the Python \`support\` extension. [Guide](docs/user-guide.md#support-tickets-az-support)`

- [ ] **Step 1: Write the three docs**
- [ ] **Step 2: Verify**

Run: `grep -n 'support-tickets-az-support' README.md docs/user-guide.md`
Expected: one hit in each file (the README link and the user-guide contents).

- [ ] **Step 3: Commit** `docs(support): document az support`

---

### Task 18: Manual verification against a real tenant

No code. Run these and report the output. Do not create a ticket without the user's explicit agreement in this session.

- [ ] **Step 1: Build and run the read-only checks**

```bash
make build
./bin/az/az support services list -o table
./bin/az/az support services problem-classifications list --service-name <a service GUID from above> -o table
./bin/az/az support in-subscription tickets list
./bin/az/az support in-subscription tickets list --filter "Status eq 'Open'"
./bin/az/az support no-subscription tickets list
./bin/az/az support in-subscription tickets show --ticket-name <existing>
./bin/az/az support in-subscription communication list --ticket-name <existing>
./bin/az/az support in-subscription chat-transcript list --ticket-name <existing>
./bin/az/az support --no-input            # prints help, no TUI
./bin/az/az support tui < /dev/null       # error: needs an interactive terminal
```

Expected: JSON arrays and objects with no errors, and the last two behave as noted.

- [ ] **Step 2: TUI, read and reply on an existing ticket**

Run `./bin/az/az support` and check:
- The list opens on the default subscription.
- `f` toggles the filter.
- `t` switches to a tenant-level scope and back.
- `Other tenant…` lists the reachable tenants.
- `enter` opens a ticket, and the thread shows in time order with HTML converted to text.
- Reply only on a ticket the user agrees to: `r`, then the body `Test reply from az-go CLI, please ignore`, then `ctrl+s`. The reply appears after the reload.

- [ ] **Step 3: Create a ticket, only with the user's agreement**

In the TUI:
1. Press `n`.
2. Title: `TEST az-go CLI - please ignore and close`.
3. Pick a billing or subscription-management problem, so it works on a Basic plan.
4. Severity `Minimal`. Accept the pre-filled contact details, and check the name, email, country, time zone and language. Attach a small `.txt` file.
   If the time zone was pre-filled as a recent CLDR-only Windows zone (for example `British Columbia Standard Time` for `America/Vancouver`) and the API rejects it, record the exact error. Then pick `Pacific Standard Time` and report the result so the mapping can be pinned to the API's accepted list.
5. Submit, and confirm the detail screen opens with a ticket number.
6. Press `s` then `y` to close the ticket.
7. Confirm with `./bin/az/az support in-subscription tickets show --ticket-name <guid>` that the status is closed and `fileWorkspaceName` is set, and with `./bin/az/az support in-subscription file list --file-workspace-name <guid>` that the file is listed.

- [ ] **Step 4: Report the results**

Report which commands passed and include the output of any that failed. No commit unless a fix was needed; a fix gets a `fix(support): …` commit.
