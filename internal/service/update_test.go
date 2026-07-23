package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
