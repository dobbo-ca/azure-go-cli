# Third-party notices

## msalruntime (Microsoft)

Windows builds of `az` embed Microsoft's `msalruntime.dll`, the native client for the
Windows Web Account Manager (WAM) broker. It is extracted to a per-user cache directory
on first use and loaded from there. It lets `az login` satisfy Conditional Access
policies that require a registered device, without a browser.

- Version: 0.20.6
- Source: the `Microsoft.Identity.Client.NativeInterop` NuGet package,
  `runtimes/win-x64/native/msalruntime.dll` and `runtimes/win-arm64/native/msalruntime_arm64.dll`
- Files: `pkg/azure/msalruntime/dll/`
- License: Microsoft Software License Terms, `pkg/azure/msalruntime/dll/LICENSE-msalruntime.txt`

No copy is embedded in the macOS or Linux builds, where the broker does not apply.

To use a different copy, set `AZ_MSALRUNTIME_DLL` to its full path, or place a
`msalruntime.dll` next to the `az` executable. Either takes precedence over the
embedded one.

The Go binding in `pkg/azure/msalruntime` is original work, written against the API
contract Microsoft publishes under the MIT license in the `javamsalruntime` sources on
Maven Central (`com.microsoft.azure:javamsalruntime:0.13.10`, `MsalRuntimeLibrary.java`).
