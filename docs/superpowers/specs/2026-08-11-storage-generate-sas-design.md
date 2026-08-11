# Design: `az storage generate-sas` (account, container, blob)

Date: 2026-08-11
Tracking: `azure-go-cli-8id`

## Summary

Add three commands, matching the Python `azure-cli` flag-for-flag:

- `az storage account generate-sas`
- `az storage container generate-sas`
- `az storage blob generate-sas`

There is no bare `az storage generate-sas` in the real CLI; `generate-sas` is always
scoped to a resource. Parity is defined against the `Azure/azure-cli` source, not the
generated reference docs, because the docs omit defaults, validators and the credential
precedence chain.

### Source of truth

All Python references below are `src/azure-cli/azure/cli/command_modules/storage/`:

| Behaviour | Location |
| --- | --- |
| account SAS body | `operations/account.py:35` |
| container SAS body | `operations/blob.py:910` |
| blob SAS body | `operations/blob.py:858` |
| flags | `_params.py:802` (account), `_params.py:1722` (container), `_params.py:980` (blob) |
| `--as-user` rules | `_validators.py:1541` |
| credential chain | `_validators.py:118` |
| datetime formats | `_validators.py:1267` |
| registration | `commands.py:128`, `commands.py:490`, `commands.py:384` |

Go SDK references are `azblob@v1.6.3` (already in `go.sum`).

## 1. Architecture

Signing splits three ways. Which path runs is decided entirely by the scope and by
`--as-user`:

```
   generate-sas
        |
   +----+--------------------------------+
   |                                     |
 account                          container / blob
   |                                     |
   | needs shared key             --as-user ?
   |                               |          |
   |                             yes          no
   v                               v          v
hand-rolled sign          user-delegation   shared key
(SDK pins services=b)     SignWithUser...   SignWith...
```

### Why the account signer is hand-rolled

`sas.AccountSignatureValues` has no `Services` field. `sas/account.go:71` writes the
literal `"b"` into the string-to-sign, and `sas/account.go:96` sets
`services: "b", // will always be "b"`. Going through the SDK would silently reduce
`--services bqtf` to blob-only, which is a correctness bug, not a missing feature.

The account string-to-sign is a documented, stable format and is visible in that same
file. Reimplementing it is about 40 lines and is fully unit-testable offline against a
fixed key, fixed clock and known-good signature.

Container and blob SAS go through the SDK unchanged. `SignWithSharedKey` and
`SignWithUserDelegation` both re-parse and re-order the permission string internally, so
this code passes raw permission letters through and does not need its own ordering logic
for those two scopes.

## 2. File layout

```
internal/storage/
+-- sas/                     <- new, shared by all three scopes
|   +-- sas.go                  permission letters, expiry parsing, IP range
|   +-- sas_test.go
|   +-- accountsig.go           hand-rolled account string-to-sign
|   +-- accountsig_test.go
|   +-- credential.go           account-key resolution chain
|   +-- credential_test.go
+-- account/generate_sas.go  <- new
+-- container/generate_sas.go<- new
+-- blob/generate_sas.go     <- new
```

Each `commands.go` gains one registration. `internal/storage/commands.go` is unchanged;
the three subgroups already exist.

The `sas` package earns its place: the expiry parser, the permission validators and the
key-resolution chain are each used by two or three of the commands. Nothing goes in there
that has a single caller.

## 3. Flag surface

Long and short forms below are exactly the Python ones. Note that `--name/-n` means
different things per scope (`_params.py:1570` and `_params.py:955`).

### Common to all three

| Flag | Notes |
| --- | --- |
| `--permissions` | Required. No `-p` short form in Python. |
| `--expiry` | Required in practice. |
| `--start` | Defaults to request time. |
| `--ip` | Single IPv4 or `ip1-ip2` range. |
| `--https-only` | Boolean. Sets protocol to `https`. |
| `--account-name` | Falls back to `AZURE_STORAGE_ACCOUNT`. |
| `--account-key` | Falls back to `AZURE_STORAGE_KEY`. |
| `--connection-string` | Falls back to `AZURE_STORAGE_CONNECTION_STRING`. |
| `--encryption-scope` | Maps to `ses`. |

### `account generate-sas` only

| Flag | Notes |
| --- | --- |
| `--services` | Required. Subset of `bqtf`. |
| `--resource-types` | Required. Subset of `sco`. |

No `--auth-mode`, no `--as-user`, no `--full-uri`. Python registers this command with
`storage_custom_command`, not the `_oauth` variant (`commands.py:128`), so those flags do
not exist there and must not exist here.

### `container generate-sas` only

| Flag | Notes |
| --- | --- |
| `--name` / `-n` | The container name. |
| `--policy-name` | Stored access policy id. Maps to `si`. |
| `--as-user` | User delegation SAS. |
| `--auth-mode` | `key` or `login`. |
| `--user-delegation-oid` | Maps to `AuthorizedObjectID` (`saoid`). |
| `--cache-control` etc. | Five content headers. |

No `--full-uri`. Python does not define it for container.

### `blob generate-sas` only

Everything container has, plus:

| Flag | Notes |
| --- | --- |
| `--name` / `-n` | The blob name. |
| `--container-name` / `-c` | The container name. |
| `--full-uri` | Return the full blob URL instead of a bare token. |
| `--snapshot` | Opaque snapshot id. |
| `--blob-url` | Full blob endpoint; an alternative to name plus container. |

### Deliberate omissions

- `--sas-token`. Python explicitly suppresses it on all three commands with
  `c.ignore('sas_token')`. Carrying it would be an anti-parity change.
- `--user-delegation-tid`. No SDK path: `generated.KeyInfo` carries only `Expiry` and
  `Start`, with no delegated-tenant field. Supporting it means hand-rolling the
  `GetUserDelegationKey` request body as well. It is a preview flag in Python. Filed as a
  follow-up bead rather than dropped.
- `--timeout`. Python's is a per-call service timeout. All three of these commands sign
  locally; only the `--as-user` path makes a network call.

### Known parity gaps

Both are SDK limits, not choices. Both get a follow-up bead rather than a workaround,
because the only workaround is hand-rolling a second signer, and that trades a rarely-hit
gap for a permanent correctness risk.

1. **`--user-delegation-tid`.** `generated.KeyInfo` has only `Expiry` and `Start`.
2. **Container permission `y`.** `sas/service.go:342` `parseContainerPermissions` accepts
   `racwdxltfmeopi` and rejects `y`; `sas.ContainerPermissions` has no `PermanentDelete`
   field, though `sas.BlobPermissions` does. Python's `ContainerSasPermissions` accepts
   `y`, and its own recorded test uses `--permissions racwdxyltfmei` on a container. The
   rejection happens inside `SignWithSharedKey`, so it cannot be bypassed by pre-ordering
   the letters. This command will reject `y` on a container with a clear message naming
   the supported set.

## 4. Credential resolution

Mirrors `_validators.py:118`, in order:

1. `--connection-string`, else `AZURE_STORAGE_CONNECTION_STRING`. When present, its
   `AccountName` and `AccountKey` override the separately supplied values, exactly as
   Python does.
2. `--account-key`, else `AZURE_STORAGE_KEY`.
3. `--account-name`, else `AZURE_STORAGE_ACCOUNT`.
4. If the account name is known and no key was found: print a warning to stderr, then
   fetch the key over ARM. The resource group is not required from the user; it is
   discovered by listing storage accounts in the subscription
   (`armstorage.NewListPager`) and matching on name, then calling `ListKeys`. A failure
   here warns and continues, matching Python's behaviour of swallowing the exception.

For `container` and `blob` with `--auth-mode login` plus `--as-user`, steps 1 to 4 are
skipped entirely. The existing `pkg/azure.GetCredential()` token is used to call
`service.Client.GetUserDelegationCredential`.

### Error text

When `account generate-sas` cannot resolve a key, the error reproduces the Python message
at `operations/account.py:39`, listing the three accepted variations. This is the single
most common failure and a vague message here costs the user real time.

## 5. Validation rules

From `_validators.py`:

- `--expiry` and `--start` accept exactly four layouts, tried in order:
  `2006-01-02T15:04:05Z`, `2006-01-02T15:04Z`, `2006-01-02T15Z`, `2006-01-02`. Anything
  else is an error naming a valid example.
- `--as-user` requires `--expiry`, requires `--auth-mode login`, and rejects an expiry
  more than **7 days** out. That 7-day cap is a service limit on the user delegation key,
  not a CLI preference.
- `--auth-mode login` without `--as-user` is an error on container and blob.
- `--user-delegation-oid` requires `--as-user`.
- Permission letters are validated against the scope before signing, so a bad letter
  fails locally with the full allowed set rather than as an opaque service error.

Permission letters, taken from the Go SDK's `String()` methods so that validation and
signing cannot drift apart:

| Scope | Letters |
| --- | --- |
| account | `rwdxylacupfti` |
| container | `racwdxltfmeopi` |
| blob | `racwdxyltmeopi` |

Container has no `y` (permanent delete). That is a real gap against Python, which accepts
it; see "Known parity gaps" below.

## 6. Output

All three return a string, and the existing `output.PrintFormatted` already renders it the
way Python does: `-o json` (the default) emits a quoted JSON string, `-o tsv` emits the
bare token. No new output code is needed.

- account: bare token.
- container: bare token.
- blob: token percent-encoded with the safe set `&%()$=',~`, matching
  `operations/blob.py:906`.
- blob with `--full-uri`: the full blob URL with the token appended, matching
  `operations/blob.py:902-905`.

## 7. Testing

Offline unit tests, no Azure account required:

| File | Covers |
| --- | --- |
| `sas/sas_test.go` | The four datetime layouts and their rejections; permission letter validation per scope; IP range parsing; the 7-day `--as-user` cap. |
| `sas/accountsig_test.go` | The hand-rolled signer against a fixed account name, fixed base64 key and fixed times, asserting an exact expected signature. Also asserts `ss=` reflects all of `bqtf`, which is the regression this signer exists to prevent. |
| `sas/credential_test.go` | The precedence chain with env vars set and unset, including connection-string override. |
| `blob/generate_sas_test.go` | `--full-uri` URL assembly and the percent-encoding safe set. |

Live verification, needing a real account, done by hand and reported:

- Round-trip each token: generate, then use it to perform the permitted operation.
- `--as-user` against a real user delegation key.
- Cross-check a generated token against the Python CLI's token for the same inputs.

## 8. Risks

- **Hand-rolled crypto.** The account signer is the only place this design writes
  signature logic. It is mitigated by a fixed-vector unit test and by the fact that the
  format is copied from the SDK's own implementation in the same repo. If the fixed-vector
  test passes, the signer is correct.
- **ARM key auto-fetch** turns a local signing command into one that makes two mgmt-plane
  calls. This matches Python, and the warning makes it visible, but it is a behaviour
  worth knowing about.
- **Account-name-to-resource-group discovery** lists every storage account in the
  subscription. On a large subscription that is slow. Python has the same cost.

## 9. Out of scope

`share`, `file`, `queue`, `table` and `fs` `generate-sas` all need SDKs not currently in
`go.mod` (`azfile`, `azqueue`, `aztable`). They are not part of this change.
