# Azure Go CLI - TODOs

## Critical Issues

### 1. Fix 403 Authorization Error
**Priority: HIGH**
**Status: In Progress**

Currently getting `AuthorizationFailed` errors when trying to list resource groups or AKS clusters:
```
"The client 'chris.dobbyn@proscia.com' with object id '9672a81a-b210-4e7e-ac2e-258a21854222' does not have authorization to perform action 'Microsoft.Resources/subscriptions/resourcegroups/read'"
```

**Known facts:**
- User has admin access to subscription (verified with official Azure CLI)
- Authentication completes successfully (device code flow works)
- MFA is triggered during login
- AuthenticationRecord is being saved and loaded correctly
- Token is cached but doesn't have required permissions/claims

**Potential causes:**
- Token scopes might not include management operations
- Tenant selection issue (organizations vs specific tenant)
- Application consent not granted for the Azure CLI public client ID
- Token audience mismatch
- Role assignments not present in token claims

**Next steps:**
- Compare token claims between official CLI and our CLI
- Check if we need to request specific scopes during authentication
- Investigate if we need to use a different authentication flow
- Test with specific tenant ID instead of "organizations"

### 2. File-Based Token Storage
**Priority: HIGH**
**Status: In Progress**

Switch from MSAL keychain storage to file-based storage to eliminate password prompts.

**Current behavior:**
- Using `cache.New(&cache.Options{Name: "msal.cache"})` stores tokens in macOS Keychain
- Requires keychain password 2-8 times per operation
- "Always Allow" doesn't work reliably

**Target behavior:**
- Store tokens in `~/.azure/` directory with 0600 permissions
- No keychain prompts
- Same security model as official Azure CLI
- Compatible with existing Azure SDK patterns

## Enhancement Backlog

### 3. Keychain Support with Better UX
**Priority: LOW**
**Status: Not Started**

After fixing authorization issues, revisit keychain storage with improved UX:
- Pre-authorize CLI in keychain to avoid repeated prompts
- Add command-line flag: `--use-keychain` vs `--use-file-cache`
- Provide clear setup instructions for keychain authorization
- Consider codesigning the binary to reduce keychain prompts

### 4. Token Refresh Testing
**Priority: MEDIUM**
**Status: Not Started**

Verify token refresh works correctly:
- Test expired token refresh
- Verify refresh tokens are used instead of re-prompting
- Test multi-tenant scenarios

### 5. Error Message Improvements
**Priority: LOW**
**Status: Not Started**

Better error messages for common issues:
- "Not authenticated" should suggest `az login`
- Authorization failures should check if user has required role
- Network errors should be more descriptive

## Completed

- ✅ Implement device code authentication flow
- ✅ Add clipboard support for device code
- ✅ Automatic browser opening
- ✅ Save AuthenticationRecord for persistent sessions
- ✅ Add AKS commands (get-credentials, list, show)
- ✅ Add resource group commands (list)
- ✅ Account management (list, show, set, clear)
- ✅ Domain-driven design structure
- ✅ MSAL persistent cache integration (with keychain issues)
