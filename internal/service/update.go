package service

import (
	"runtime/debug"

	"github.com/marcoarnulfo/clickup-cli/internal/version"
)

// ldflagsVersion can be set at build time with
//
//	-ldflags "-X github.com/marcoarnulfo/clickup-cli/internal/service.ldflagsVersion=v1.8.0"
//
// No tooling sets it today: it exists so a future release pipeline can stamp a
// version, and as the injection seam that keeps version.Resolve testable.
var ldflagsVersion string

// CurrentVersion reports the version of the running binary. For a `go install
// module/cmd@vX.Y.Z` build the Go toolchain stamps the resolved tag, so this
// is a real release version; for a local `go build` it is a pseudo-version,
// which version.IsRelease deliberately rejects.
func CurrentVersion() string {
	main := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		main = info.Main.Version
	}
	return version.Resolve(ldflagsVersion, main)
}
