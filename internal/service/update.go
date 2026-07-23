package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

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

// updateCheckInterval is how long a check result is reused before asking
// GitHub again.
const updateCheckInterval = 24 * time.Hour

type updateCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// defaultCachePath returns the cache location. os.UserCacheDir honours
// XDG_CACHE_HOME on Linux and gives the right directory on macOS, mirroring
// how internal/config uses os.UserConfigDir rather than a hardcoded path.
func defaultCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "clup", "update.json"), nil
}

// readCache reads the cached result. A missing, unreadable or malformed file
// reports ok == false: the caller treats that as stale and refetches. A cache
// problem is never surfaced to the user.
func readCache(path string) (updateCache, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return updateCache{}, false
	}
	var c updateCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return updateCache{}, false
	}
	return c, true
}

// writeCache writes the cache atomically: a temp file in the same directory
// followed by a rename, so a concurrent reader sees either the old file or the
// new one and never a truncated one.
func writeCache(path string, c updateCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeded
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// cacheFresh reports whether the cached result is still usable. A CheckedAt in
// the future counts as stale: without that, a clock moved backwards would keep
// the cache "fresh" forever.
func cacheFresh(c updateCache, now time.Time) bool {
	if c.CheckedAt.IsZero() || c.CheckedAt.After(now) {
		return false
	}
	return now.Sub(c.CheckedAt) < updateCheckInterval
}
