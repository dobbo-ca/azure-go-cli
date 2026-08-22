//go:build windows && amd64

package msalruntime

import _ "embed"

// embeddedDLL is Microsoft's msalruntime.dll for windows/amd64, taken from the
// Microsoft.Identity.Client.NativeInterop NuGet package. Only the matching
// architecture is embedded, so a build carries one copy, not both.
//
// See dll/LICENSE-msalruntime.txt for its license terms.
//
//go:embed dll/msalruntime_amd64.dll
var embeddedDLL []byte
