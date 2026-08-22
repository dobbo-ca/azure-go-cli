//go:build windows && arm64

package msalruntime

import _ "embed"

// embeddedDLL is Microsoft's msalruntime.dll for windows/arm64, taken from the
// Microsoft.Identity.Client.NativeInterop NuGet package. Only the matching
// architecture is embedded, so a build carries one copy, not both.
//
// See dll/LICENSE-msalruntime.txt for its license terms.
//
//go:embed dll/msalruntime_arm64.dll
var embeddedDLL []byte
