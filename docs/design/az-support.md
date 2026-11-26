# az support: support tickets with a TUI

Package: `internal/support`. User docs: [docs/user-guide.md](../user-guide.md#support-tickets-az-support). Original spec and plan: [spec](../superpowers/specs/2026-11-23-az-support-design.md), [plan](../superpowers/plans/2026-11-24-az-support.md).

## Purpose

Two surfaces over the same code: a full-screen bubbletea TUI to list, read, reply to and create support tickets for any subscription or tenant, and flag commands that match the Python `support` extension (names, required flags, enums).

The Python extension has no interactive mode and no tenant-level switcher. Filing a ticket there needs service and problem-classification GUIDs; here a local search finds them.

## Layout

One flat package. A `tui/` subpackage would need to import `support` for the `Client`, `createTicket` and `uploadFile`, while `support` imports it to launch the TUI, which is an import cycle. The TUI files are `tui_*.go` in the same package.

```
cobra cmd (commands.go)  ----------------+
  tickets / communication / files / ...    |   both call the same helpers
tui_app.go (bubbletea root)  --------------+   through Client
  list -> detail -> reply, create, scope   |
                                           v
  createTicket, uploadFile, loadCatalog, rank
                    |
                 Client (client.go)
        subscriptionClient   tenantClient
        (armsupport v2 clients, API 2024-04-01)
```

## Key pieces

| Piece | File | Role |
|---|---|---|
| `Client` | `client.go` | Interface over the armsupport clients, only the methods used. `subscriptionClient` and `tenantClient` (no-subscription) are the two implementations; fakes implement it in tests. |
| `Scope` | `scope.go` | Tenant ID plus optional subscription ID. `TenantLevel()` is true with no subscription. |
| `createTicket` | `tickets.go` | The shared create path for the flag command and the TUI. |
| `readUpload`, `uploadFile` | `files.go` | Python's upload rules and chunking. |
| `loadCatalog`, `rank` | `services.go`, `search.go` | Local service / problem search. |
| `list`, `pageToken` | `paging.go` | Shared list paging for `--max-items` / `--next-token`. |
| `contactFromMe` | `contact.go` | Contact defaults from Graph `/me`, locale and the local zone. |
| `app`, `screen`, `deps` | `tui_app.go` | Root model, screen stack and injected dependencies. |

## Create order

`createTicket` stops at the first error and does not roll back:

1. Validate the problem classification ID and derive `serviceId` from it. Default the start time to now (UTC).
2. `CheckName` for the ticket.
3. Unless a workspace was given, create one named after the ticket, or a GUID when the ticket name does not match the workspace name pattern, after `CheckName` on it.
4. Upload the attachment, if any.
5. Create the ticket with `fileWorkspaceName` set.

An API links a workspace to a ticket only at create time, so every ticket made here gets one. An empty workspace expires on its own. In the TUI, `readUpload` runs on leaving the attach step, so a bad file is rejected before submit.

## Local search

The preview classifier APIs are not used (removed from the Go SDK before GA). `loadCatalog` lists every service and, concurrently (8 at a time), its problem classifications, and the app caches the rows for the process. `rank` keeps rows containing at least one query word, best match first, ties in input order. The query starts as the distinctive words of the title (`queryFromTitle`).

## Time zone and country data

The API wants Windows time zone names and alpha-3 country codes. `cldr/windowsZones.xml` and `cldr/timezone.xml` (Unicode CLDR `release-49`) are embedded and parsed once (`tzdata.go`), mapping IANA zones, including legacy aliases and a macOS `/etc/localtime` link, to Windows zones. Countries come from `golang.org/x/text/language`. The CLDR data is under the Unicode license in `cldr/LICENSE`. A recent CLDR-only Windows zone name may be rejected by the API; the contact step lets the user pick another.

## Stale scope results

Every async TUI message carries `app.gen`, which `setScope` increments. Screens drop a message whose `gen` differs, so a slow fetch for the old scope cannot replace the new scope's list. The same guard exists a level down: a list load carries its filter, and a detail load carries its ticket name.

## Paging

`--next-token` uses Python's token, so tokens work in either CLI: base64 of `{"next_link": <URL of the page that was cut, or null for the first page>, "offset": <index in that page of the first item not shown>}`. `list` (`paging.go`) takes the first page of a query from the SDK pager and fetches every other page by link through an `arm.Client` pipeline built from the same credential and options, because the armsupport pagers cannot start from a link. When `--max-items` cuts the output it writes `WARNING: Token of next page: <token>` to stderr; a cut at the end of a page with a `nextLink` gives `{next_link: nextLink, offset: 0}` without fetching that page. As in Python, the token is used only with `--max-items`, and a first-page token (null `next_link`) needs the same `--filter` to resume. A token's `next_link` must be `https` on the ARM endpoint host and in normalized form, or it is rejected before any request carries the bearer token.

## Sign-in

`ctrl+l` and the scope switcher sign in through `tea.Exec` with a `signInCommand`: bubbletea releases the terminal, the browser sign-in runs, and the TUI resumes. Nothing the user types is written to disk and the profile is never written.

## Decisions

- **No TUI under scripts.** `--no-input`, a non-TTY stdin or stdout, or any subcommand never starts the TUI; `az support tui` errors without a terminal.
- **API errors are shown verbatim**, in the TUI and from the flag commands.
- **No `tea.Sequence`, no blinking cursors** in screens. Their messages are unexported or re-arm forever, which would stall the test driver (`drive` in `fake_test.go`).
- **Static "Loading…" text instead of a spinner.** A spinner re-arms a timer Cmd on every tick, which the same driver cannot finish.
- **go.mod `go` directive is 1.26.0**, because the charm modules require it. The release workflow pins `go-version: '1.23'` and relies on toolchain auto-download, as it already does for 1.25.2.
