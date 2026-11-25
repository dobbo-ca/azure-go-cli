# `az support` — Azure support tickets with a TUI

Date: 2026-11-23
Status: draft (pending builder check and user approval)

## Intent

The Azure portal puts many pages and gates in front of "create a support ticket":
a service picker, a problem-type tree, a "solutions" detour, diagnostics pages, and
then the form. The effect is that filing a ticket is slow and discouraging. This
CLI should let a user file a ticket against a chosen tenant or subscription in one
screen-flow, and also list, read, and reply to their existing tickets, without
opening the portal.

Two surfaces:

1. **An interactive TUI** (`az support` with no subcommand on a TTY). This is the
   unique capability — the Python CLI has nothing like it.
2. **Flag-based commands** with the same names and flags as the Python
   `support` extension, for scripts and parity.

## Goals

- Start on the active profile's scope, and switch to any subscription or tenant
  (tenant-level, "no subscription" ticket) from inside the TUI.
- Create a ticket in as few steps as the API allows: describe the issue, pick a
  service and problem type from a searchable list, choose severity, confirm contact
  details (pre-filled), submit.
- List tickets for the scope, open one, read the full message thread, reply, and
  attach a file to a new ticket or a reply.
- Change status (close / reopen) and severity of an existing ticket.
- Full parity with the Python `support` extension command tree (see "Flag commands").

## Non-goals

- Using the preview classifier APIs (`classifyServices`, `classifyProblems`,
  `2025-06-01-preview`). Microsoft removed them from the Go SDK in
  `v2.0.0-beta.4` (2026-03-16) before GA; they are not stable. (User chose local
  search, "option A".)
- Quota-increase tickets in the TUI. They need structured `quotaTicketDetails`
  payloads per quota type; `az quota` already covers quota requests. The flag
  command still accepts the Python quota flags. In the TUI, choosing the quota
  service shows a one-line hint pointing to the flag command / `az quota`.
- Problem scoping questions, secondary consent, and support-engineer chat in the
  TUI. Chat transcripts are readable via the flag commands only.
- Persisting contact details or any other config (no new config file).
- Bypassing support-plan entitlements. A Basic plan can only file billing,
  subscription-management, and quota tickets; the API enforces this. The TUI
  shows the API error text verbatim.

## Anti-goals

- A TUI that is slower than the portal for the common case, or that shows a wall
  of required fields up front. Every field that can be pre-filled is pre-filled.
- Silent failure: an API error must be visible in the TUI with its message.
- Breaking scripted use: with `--no-input`, a non-TTY stdin, or any subcommand,
  no TUI starts.

## Constraints (from recon)

- SDK: `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2`
  `v2.0.0` (latest, API `2024-04-01`). Clients: `TicketsClient`,
  `TicketsNoSubscriptionClient`, `CommunicationsClient`,
  `CommunicationsNoSubscriptionClient`, `ServicesClient`,
  `ProblemClassificationsClient` (no subscription needed), `ChatTranscripts*`,
  `FileWorkspaces*`, `Files*`.
- Ticket create needs: `title`, `description`, `serviceId`,
  `problemClassificationId`, `severity` (`minimal|moderate|critical|highestcriticalimpact`),
  `advancedDiagnosticConsent` (`Yes|No`), and `contactDetails` with
  `firstName`, `lastName`, `primaryEmailAddress`, `preferredContactMethod`
  (`email|phone`), `preferredTimeZone` (Microsoft Time Zone Index name, e.g.
  `Pacific Standard Time`), `preferredSupportLanguage` (one of 11 codes, e.g.
  `en-us`), `country` (ISO 3166-1 alpha-3). `phoneNumber` is required for phone
  contact and for `critical`/`highestcriticalimpact` severity.
- `serviceId` is derivable from the problem classification ID
  (`/providers/Microsoft.Support/services/{svc}/problemClassifications/{pc}`), as
  the Python extension does.
- Ticket and communication names should be checked with `CheckNameAvailability`
  before create (Python does this).
- Ticket list filter: OData; `Status`, `ServiceId`, `ProblemClassificationId` with
  `eq`; `CreatedDate` with `gt`/`ge`; `Top` max 100. Communications `Top` max 10
  per page (use the pager).
- File upload (Python `_utils.upload_file`): blocklisted extensions, stem ≤ 110
  chars, size 1 B – 5 MiB, chunk size `min(2.5 MiB, size)`, base64 chunks sent in
  order after a `file create` with size/chunk metadata; the workspace is created
  first with name-availability type `Microsoft.Support/fileWorkspaces`.
- Repo conventions: credential for a given tenant is
  `azure.TenantCredential(tenantID, authRecord)` (`pkg/azure/authmode.go`);
  profile subscriptions with `TenantID` come from `config.Load()`; output via
  `output.PrintJSON(cmd, data)`; TTY detection as in `internal/pim/prompt.go`
  (`term.IsTerminal` + `--no-input`); register in `cmd/az/main.go`; new
  packages use tabs (gofmt), gofmt only our own files.

## Approaches considered

TUI style:

- **A. Full-screen app (bubbletea).** Chosen. List, detail, reply, and create are
  screens in one program, so the user can move between tickets and reply without
  re-running commands.
- B. Step-by-step `promptui` prompts (as in `az login`). No new dependency, but
  linear; it cannot browse and reply from one place.

Finding the service and problem type:

- **A. Local search over the GA lists.** Chosen by the user. Load all services
  and their problem classifications once (concurrently), then filter on the
  user's words.
- B. Preview classifier APIs. Better suggestions, but preview-only and already
  removed from the Go SDK once.

## Command tree

```
az support                                      # TUI on a TTY; else help
az support tui                                  # TUI explicitly (errors if no TTY)
az support services list|show
az support services problem-classifications list|show
az support in-subscription tickets create|list|show|update
az support in-subscription communication create|list|show
az support in-subscription chat-transcript list|show
az support in-subscription file upload|list|show
az support in-subscription file-workspace create|show
az support no-subscription <same groups as in-subscription>
```

Flag names, required-ness and enums match the Python extension exactly (recon
listing is the source; read `azext_support` and its `aaz/` tree when in doubt, do
not recall). Notable behaviours copied from Python:

- `tickets create` derives `serviceId` from `--problem-classification`, validates
  the ID shape, defaults `--start-time` to now (UTC), checks name availability,
  and for technical services validates `--technical-resource` is in the current
  subscription.
- `tickets list` with no `--filter` uses `CreatedDate ge <today-7d>`.
- `communication create` checks name availability first.
- `file upload` follows the chunking and validation rules above.

Python's internal-only commands (`check-name-availability`, `file create`, the
raw chunk `file upload`) are not exposed; their logic is internal helpers.
`communication update` and `file update` exist in the aaz tree only and are
skipped (not user-facing in Python's help).

Output: `-o json` default like the rest of the CLI; list commands keep JSON
default (Python parity), `-o table` works through the generic formatter.

## TUI design

```
 ticket list ──enter──> ticket detail
  │ n: new   t: scope    │  r: reply   a: attach
  │ f: filter            │  s: status  v: severity
  v                      v
 new-ticket flow       reply editor ──ctrl+s──> send
```

Library: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2` (list, textinput,
textarea, viewport, spinner), `charm.land/lipgloss/v2`. Builder picks the exact
latest stable versions.

### Scope (user decision: start on the active profile, switch on demand)

- The TUI opens straight on the ticket list for the **active profile's** default
  subscription (the profile selected by `AZ_SESSION`, if set). No picker first.
  `--subscription` (root flag) or `--tenant` (new flag on `az support` / `tui`;
  tenant-level scope) sets the starting scope instead.
- The header always shows the current scope (tenant + subscription, or
  "tenant-level").
- `t` opens the scope switcher:
  - Rows from `config.Load().Subscriptions`, grouped by tenant; each tenant group
    starts with a "Tenant-level tickets (no subscription)" row. Type-to-filter.
    No network call. Tenant headings use the tenant ID (the profile stores no
    tenant names).
  - A final row "Other tenant…" loads every tenant the user can reach, using the
    existing login discovery (`pkg/azure/discovery.go`, which returns display
    names and `NeedsMFA`). Spinner while loading. Picking a tenant that needs MFA
    runs the same tenant-targeted sign-in `az login` uses
    (`AuthenticateForTenant`), then lists its subscriptions plus a tenant-level
    row.
  - Switching scope does not change the profile's default subscription and does
    not write the profile. (If discovery/sign-in must update the token cache,
    that is fine; that is the cache's job.)
- The credential is `TenantCredential(tenantOfSelection, authRecord)`. If it fails
  with an MFA/consent error, show the error and offer the same tenant sign-in.

### Ticket list

- Default filter: open tickets (`Status eq 'Open'`); `f` toggles to "all,
  last 30 days" (`CreatedDate ge ...`). Columns: created date, severity, status,
  title, ticket number (`supportTicketId`). Sorted newest first.
- `n` new ticket, `enter` open, `t` switch scope, `/` search the loaded rows,
  `ctrl+r` refresh, `q` quit.

### Ticket detail

- Header: title, ticket number, status, severity, service / problem display
  names, created/modified time, support plan.
- Body: description, then the communications thread (all pages, oldest first),
  with direction (inbound = from Microsoft, outbound = from you), sender, time,
  subject, body. HTML bodies from Microsoft are converted to plain text
  (strip tags, decode entities, keep line breaks).
- Keys: `r` reply, `a` attach a file to the ticket's workspace, `s`
  close/reopen (confirm), `v` change severity (picker; phone number prompt if
  needed), `esc` back.

### Reply editor

- Textarea for the body; subject pre-filled with the ticket title
  (editable). Optional attachment path. `ctrl+s` sends: name check → create
  communication (poller) → refresh thread. Communication name is a new GUID.

### New-ticket flow

One screen per step, `esc` goes back one step, values are kept:

1. **Describe:** title and description (textarea). Optional problem start time
   (default now).
2. **Service and problem type:** a single search list. On entering the flow the
   TUI loads `services list` and, concurrently, `problem-classifications list`
   for every service (cache for the life of the process). Each row is
   "Service › Problem type". The search box is pre-filled with words from the
   title; the user can edit it. Ranking: rows that match all words first, then by
   count of matched words; match on service and problem display names. The full
   list is reachable by clearing the search. While loading, show a spinner and
   allow typing.
3. **Resource (technical services only):** if the service lists resource types
   (`Service.Properties.ResourceTypes`) and the scope is a subscription, offer a
   picker of resources of those types in the subscription
   (`armresources` list with a resourceType filter), plus "skip" and "enter ID".
   Billing / subscription-management / quota services skip this step (same
   service-ID constants as Python). Quota service shows the non-goal hint.
4. **Severity and consent:** severity picker (minimal / moderate / critical /
   highest critical impact) with the plan caveat in help text;
   24x7 toggle when severity is critical or above; advanced diagnostic consent
   Yes/No (default Yes, as the portal does).
5. **Contact:** pre-filled, editable fields:
   - first/last name and email: Microsoft Graph `GET /me` (`givenName`,
     `surname`, `mail`, falling back to `userPrincipalName`) with a Graph token
     from the same credential (precedent: `internal/keyvault/principal.go`). If
     Graph fails, fields start empty.
   - country: Graph `usageLocation` (alpha-2) mapped to alpha-3; picker of
     countries if unknown.
   - time zone: local IANA zone (`time.Local` / `TZ`) mapped to the Windows
     zone name with an embedded CLDR `windowsZones` table; picker of Windows
     zone names if unknown.
   - language: from `LANG`/`LC_ALL` mapped to the 11 supported codes, default
     `en-us`; picker.
   - contact method: email (default) or phone; phone number field shown when
     phone or severity critical+.
   - additional emails: optional, comma-separated.
6. **Attach (optional):** file path; validated with the Python rules before
   submit.
7. **Review and submit:** summary of everything; `enter` submits: ticket name =
   new GUID → name check → file workspace create (always, named with the
   ticket GUID) → upload if attaching → `BeginCreate` poller with spinner, with
   `fileWorkspaceName` set → open the new ticket's detail screen.

**Attachments and file workspaces (builder-check finding, my call).** The API
links a workspace to a ticket only at create time (`UpdateSupportTicket` has no
workspace field). So every ticket this CLI creates — TUI or flags — gets a
workspace, which keeps later attachments possible. The flag `tickets create`
creates one only when `--file-workspace-name` is not given (a deviation from
Python, which creates none; it adds no required input). For a ticket with no
`fileWorkspaceName` (for example, some portal tickets), `a` and the reply
attachment field show "this ticket has no file workspace" instead of failing.

Nothing typed is written to disk. On API error the flow stays on the review
screen with the error shown, so the user can fix and resubmit.

## Package layout (proposal; builder may adjust)

```
internal/support/
  commands.go        cobra tree, TTY decision
  scope.go           scope type (tenant ID + optional subscription ID), clients
  tickets.go         create/list/show/update (flags), shared helpers
  communication.go   create/list/show
  chat.go            chat-transcript list/show
  files.go           file-workspace create/show, file upload/list/show, chunking
  services.go        services + problem-classifications
  contact.go         Graph /me, locale/time zone/country defaults
  tzdata.go          embedded IANA -> Windows zone map, alpha-2 -> alpha-3 map
  search.go          service/problem ranking
  htmltext.go        HTML -> plain text for message bodies
  tui/               bubbletea models: app, scope, list, detail, reply, create
```

A small internal interface over the SDK clients (only the methods used) lets the
TUI and flag commands share code and lets tests use fakes. One interface, two
implementations (subscription / no-subscription) — this is the real variation,
not speculative.

## Testing

- Unit tests: problem-classification ID parsing/serviceId derivation, ticket
  list default filter, file validation and chunk math, search ranking, HTML to
  text, time zone/country/language mapping, technical-service detection.
- TUI model tests: drive `Update` with key messages against fake clients and
  assert on state/view (create flow end-to-end to the create call with expected
  `TicketDetails`; reply sends expected `CommunicationDetails`).
- Manual test against a real tenant: list/show/reply on an existing ticket; a
  real create only with the user's agreement (creates a real ticket — use
  severity minimal and a clearly-labelled test title, then close it).

## Docs

- `docs/user-guide.md`: new section "Support tickets (az support)" with the TUI
  walkthrough and flag examples; link from the README capabilities list.
- `docs/design/az-support.md`: implementation design (from this spec, updated
  to match what was built).

## Left to the builder

- Exact bubbletea/bubbles/lipgloss versions and key bindings beyond those above.
- Visual styling.
- Concurrency limit when loading problem classifications (keep it modest, e.g. 8).
- Whether the `ProblemClassificationsClient` load is lazy per service or eager
  (eager is the default here, for search).
