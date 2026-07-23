package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
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

// githubLatestReleaseURL is the endpoint asked for the newest release.
//
// It deliberately excludes drafts and prereleases. That is what keeps the
// release flow quiet in the window where a tag is pushed but its notes are
// still a draft: the endpoint keeps returning the previous release. Do not
// swap it for the tags API, which would lose that property.
const githubLatestReleaseURL = "https://api.github.com/repos/marcoarnulfo/clickup-cli/releases/latest"

const updateCheckTimeout = 2 * time.Second

// UpdateCheckEnabled reports whether the update check may run.
// CLUP_NO_UPDATE_CHECK wins over the config; demo mode never checks, because a
// demo session performs no I/O at all.
func UpdateCheckEnabled(cfg config.Config, demo bool) bool {
	if demo {
		return false
	}
	if os.Getenv("CLUP_NO_UPDATE_CHECK") != "" {
		return false
	}
	if cfg.UpdateCheck != nil && !*cfg.UpdateCheck {
		return false
	}
	return true
}

// UpdateOptions configures a check. The zero value of each optional field
// selects the production default; tests fill them in.
type UpdateOptions struct {
	Current   string    // current version; the check does not run unless it is a release
	CachePath string    // "" => defaultCachePath()
	APIURL    string    // "" => githubLatestReleaseURL
	Now       time.Time // zero => time.Now()
}

// CheckForUpdate reports the latest published release and whether it is
// strictly newer than the running version.
//
// It never returns an error: every failure — no cache, a corrupt cache, no
// network, a timeout, a non-200, a malformed body — is silent, because a
// failed update check is not the user's problem.
func CheckForUpdate(ctx context.Context, o UpdateOptions) (string, bool) {
	// A source build has nothing meaningful to compare against, and must not
	// even reach the network.
	if !version.IsRelease(o.Current) {
		return "", false
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	path := o.CachePath
	if path == "" {
		p, err := defaultCachePath()
		if err != nil {
			return "", false
		}
		path = p
	}

	cached, ok := readCache(path)
	if ok && cacheFresh(cached, now) {
		return cached.Latest, version.Newer(o.Current, cached.Latest)
	}

	latest, err := fetchLatestRelease(ctx, o)
	if err != nil || !version.IsRelease(latest) {
		// Keep whatever we knew, but record the attempt so an offline user
		// does not pay the timeout on every invocation.
		latest = cached.Latest
	}
	// A write failure is deliberately ignored: on an unwritable cache dir the
	// "at most one call per 24h" guarantee degrades to a fetch per invocation,
	// still silent and still 2s-bounded. A courtesy feature does not warrant
	// surfacing a cache error, and the trade-off is accepted knowingly.
	_ = writeCache(path, updateCache{CheckedAt: now, Latest: latest})
	return latest, version.Newer(o.Current, latest)
}

func fetchLatestRelease(ctx context.Context, o UpdateOptions) (string, error) {
	url := o.APIURL
	if url == "" {
		url = githubLatestReleaseURL
	}
	client := &http.Client{Timeout: updateCheckTimeout}
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// GitHub rejects requests without a User-Agent. The call carries no
	// Authorization header: it is anonymous, and the user's ClickUp token has
	// no business travelling to api.github.com.
	req.Header.Set("User-Agent", "clup/"+o.Current)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	return body.TagName, nil
}
