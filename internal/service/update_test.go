package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if err := writeCache(path, updateCache{CheckedAt: now, Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	got, ok := readCache(path)
	if !ok || got.Latest != "v1.8.0" || !got.CheckedAt.Equal(now) {
		t.Fatalf("readCache = %+v, ok=%v", got, ok)
	}
}

func TestReadCacheMissingOrCorrupt(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readCache(filepath.Join(dir, "nope.json")); ok {
		t.Error("missing cache must not report ok")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCache(bad); ok {
		t.Error("corrupt cache must not report ok — it is treated as stale, never as an error")
	}
}

func TestCacheFresh(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		checkedAt time.Time
		want      bool
	}{
		{"just checked", now.Add(-time.Minute), true},
		{"23h ago", now.Add(-23 * time.Hour), true},
		{"25h ago", now.Add(-25 * time.Hour), false},
		{"exactly 24h ago", now.Add(-24 * time.Hour), false},
		{"in the future", now.Add(time.Hour), false}, // clock moved back: never "fresh forever"
		{"zero value", time.Time{}, false},
	}
	for _, c := range cases {
		if got := cacheFresh(updateCache{CheckedAt: c.checkedAt}, now); got != c.want {
			t.Errorf("%s: cacheFresh = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWriteCacheLeavesNoTempFile(t *testing.T) {
	// Two concurrent clup invocations can write the cache at the same time; a
	// truncated file is exactly the corruption readCache has to tolerate.
	// Writing through a temp file plus rename means a reader sees either the
	// old file or the new one, never half of one.
	//
	// Note the limit of this test, and do not over-trust it: it only proves no
	// temp file is left behind. A plain os.WriteFile would pass it too.
	// Atomicity itself is not practically assertable here; the guarantee comes
	// from os.Rename, and the test guards the litter the technique produces.
	dir := t.TempDir()
	path := filepath.Join(dir, "update.json")
	if err := writeCache(path, updateCache{CheckedAt: time.Now(), Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "update.json" {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestUpdateCheckEnabled(t *testing.T) {
	no, yes := false, true
	cases := []struct {
		name string
		env  string
		cfg  config.Config
		demo bool
		want bool
	}{
		{"default on", "", config.Config{}, false, true},
		{"config nil is on", "", config.Config{UpdateCheck: nil}, false, true},
		{"config true", "", config.Config{UpdateCheck: &yes}, false, true},
		{"config false", "", config.Config{UpdateCheck: &no}, false, false},
		{"env wins over config true", "1", config.Config{UpdateCheck: &yes}, false, false},
		{"env any value", "please-dont", config.Config{}, false, false},
		{"demo never checks", "", config.Config{UpdateCheck: &yes}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CLUP_NO_UPDATE_CHECK", c.env)
			if got := UpdateCheckEnabled(c.cfg, c.demo); got != c.want {
				t.Errorf("UpdateCheckEnabled = %v, want %v", got, c.want)
			}
		})
	}
}

// newTestAPI returns a server answering like GitHub's releases/latest, and a
// counter of how many requests it received.
func newTestAPI(t *testing.T, tag string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("User-Agent") == "" {
			t.Error("request has no User-Agent; GitHub rejects those")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("update check must never send an Authorization header")
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCheckForUpdateFetchesAndReportsNewer(t *testing.T) {
	srv, calls := newTestAPI(t, "v1.8.0", http.StatusOK)
	latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current:   "v1.7.0",
		CachePath: filepath.Join(t.TempDir(), "update.json"),
		APIURL:    srv.URL,
		Now:       time.Now(),
	})
	if latest != "v1.8.0" || !newer {
		t.Fatalf("got (%q, %v), want (v1.8.0, true)", latest, newer)
	}
	if *calls != 1 {
		t.Fatalf("calls = %d, want 1", *calls)
	}
}

func TestCheckForUpdateUsesFreshCacheWithoutCallingTheServer(t *testing.T) {
	// Without the request counter this test would prove nothing: it would pass
	// whether or not the cache was consulted.
	srv, calls := newTestAPI(t, "v9.9.9", http.StatusOK)
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if err := writeCache(path, updateCache{CheckedAt: now.Add(-time.Hour), Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: path, APIURL: srv.URL, Now: now,
	})
	if latest != "v1.8.0" || !newer {
		t.Fatalf("got (%q, %v), want the cached (v1.8.0, true)", latest, newer)
	}
	if *calls != 0 {
		t.Fatalf("server was called %d times despite a fresh cache", *calls)
	}
}

func TestCheckForUpdateSkipsNonReleaseCurrent(t *testing.T) {
	srv, calls := newTestAPI(t, "v1.8.0", http.StatusOK)
	for _, current := range []string{"dev", "(devel)", "v1.6.1-0.20260723143812-50d39f8", "v1.7.0+dirty"} {
		latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
			Current: current, CachePath: filepath.Join(t.TempDir(), "update.json"),
			APIURL: srv.URL, Now: time.Now(),
		})
		if newer || latest != "" {
			t.Errorf("current=%q: got (%q, %v), want no check at all", current, latest, newer)
		}
	}
	if *calls != 0 {
		t.Fatalf("a source build must not reach the network (calls=%d)", *calls)
	}
}

func TestCheckForUpdateOlderOrEqualIsSilent(t *testing.T) {
	for _, tag := range []string{"v1.7.0", "v1.6.0"} {
		srv, _ := newTestAPI(t, tag, http.StatusOK)
		_, newer := CheckForUpdate(context.Background(), UpdateOptions{
			Current: "v1.7.0", CachePath: filepath.Join(t.TempDir(), "update.json"),
			APIURL: srv.URL, Now: time.Now(),
		})
		if newer {
			t.Errorf("tag %q: reported an update over v1.7.0", tag)
		}
	}
}

func TestCheckForUpdateFailuresAreSilentAndStampTheCache(t *testing.T) {
	// Offline users must not pay the timeout on every single invocation, so a
	// failed attempt still records when it happened.
	srv, _ := newTestAPI(t, "", http.StatusInternalServerError)
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if _, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: path, APIURL: srv.URL, Now: now,
	}); newer {
		t.Fatal("a 500 must not produce a notice")
	}
	c, ok := readCache(path)
	if !ok || !c.CheckedAt.Equal(now) {
		t.Fatalf("failed attempt not stamped: %+v ok=%v", c, ok)
	}
}

func TestCheckForUpdateFailureKeepsPreviousLatest(t *testing.T) {
	srv, _ := newTestAPI(t, "", http.StatusInternalServerError)
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if err := writeCache(path, updateCache{CheckedAt: now.Add(-48 * time.Hour), Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: path, APIURL: srv.URL, Now: now,
	})
	// An offline user who already learned about v1.8.0 keeps being told: it is
	// still true.
	if latest != "v1.8.0" || !newer {
		t.Fatalf("got (%q, %v), want the previously known (v1.8.0, true)", latest, newer)
	}
}

func TestCheckForUpdateMalformedBodyIsSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{not json")
	}))
	defer srv.Close()
	if _, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: filepath.Join(t.TempDir(), "update.json"),
		APIURL: srv.URL, Now: time.Now(),
	}); newer {
		t.Fatal("malformed JSON must not produce a notice")
	}
}
