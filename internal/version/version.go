// Package version reports and compares clup's build version. It is pure: no
// I/O, no time, no dependencies beyond the standard library.
package version

import "strings"

// Dev is the version reported when no build version is available.
const Dev = "dev"

// parseRelease parses a published release version — "vMAJOR.MINOR.PATCH" with
// no prerelease segment and no build metadata — into its three components.
//
// The shape test is deliberately positive rather than a list of exclusions.
// Since Go 1.24 a local `go build` stamps the version from VCS state, so a
// source build past a tag reports a pseudo-version such as
// "v1.6.1-0.20260723143812-50d39f8" (and "+dirty" on a dirty tree), not
// "(devel)". Enumerating the forms to reject would be incomplete by
// construction; requiring the release shape is not.
func parseRelease(v string) (major, minor, patch int, ok bool) {
	rest, found := strings.CutPrefix(v, "v")
	if !found {
		return 0, 0, 0, false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var out [3]int
	for i, p := range parts {
		n, ok := atoiStrict(p)
		if !ok {
			return 0, 0, 0, false
		}
		out[i] = n
	}
	return out[0], out[1], out[2], true
}

// atoiStrict parses a run of ASCII digits. Unlike strconv.Atoi it rejects
// signs, so "+7" (which Atoi would happily read as 7) is not a version
// component.
func atoiStrict(s string) (int, bool) {
	if s == "" || len(s) > 9 { // 9 digits keeps the accumulation far from overflow
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// IsRelease reports whether v is a published release version. Only these
// versions take part in an update check: a source build must never be told to
// "update".
func IsRelease(v string) bool {
	_, _, _, ok := parseRelease(v)
	return ok
}

// Newer reports whether latest is strictly newer than current. It is false
// unless both are release versions — and false when they are equal, or when
// latest is older, so that a locally built pre-release tag is never told to
// downgrade.
func Newer(current, latest string) bool {
	cMaj, cMin, cPatch, ok := parseRelease(current)
	if !ok {
		return false
	}
	lMaj, lMin, lPatch, ok := parseRelease(latest)
	if !ok {
		return false
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPatch > cPatch
}

// Resolve picks the version to report, preferring a value injected at build
// time, then the version the Go toolchain stamped into the binary, then Dev.
// It takes both inputs as parameters so it can be tested: debug.ReadBuildInfo
// inside a test binary reports the test module, not the built program.
func Resolve(ldflagsVersion, mainVersion string) string {
	if ldflagsVersion != "" {
		return ldflagsVersion
	}
	if mainVersion != "" {
		return mainVersion
	}
	return Dev
}
