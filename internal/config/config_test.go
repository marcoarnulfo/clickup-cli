package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfig redirects the config to a temp dir on ALL platforms.
// On macOS os.UserConfigDir uses $HOME/Library/Application Support
// (ignoring XDG_CONFIG_HOME); on Linux it uses XDG_CONFIG_HOME. We set both,
// so tests NEVER touch the user's real config.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CLICKUP_TOKEN", "") // avoid override during tests
	return dir
}

// withPaths overrides the injectable configPath/legacyConfigPath vars to
// point at the given files, restoring the originals on test cleanup. It also
// clears CLICKUP_TOKEN so tests control the env override explicitly.
func withPaths(t *testing.T, newPath, legacyPath string) {
	t.Helper()
	origNew, origLegacy := configPath, legacyConfigPath
	configPath = func() (string, error) { return newPath, nil }
	legacyConfigPath = func() (string, error) { return legacyPath, nil }
	t.Cleanup(func() {
		configPath = origNew
		legacyConfigPath = origLegacy
	})
	t.Setenv("CLICKUP_TOKEN", "")
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	isolateConfig(t)
	want := Config{
		Token: "tok_123", WorkspaceID: "900", Currency: "EUR", Rate: 45,
		Rates: map[string]float64{"111": 60, "222": 30},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != want.Token || got.WorkspaceID != want.WorkspaceID ||
		got.Currency != want.Currency || got.Rate != want.Rate {
		t.Fatalf("scalar round-trip mismatch: got %+v want %+v", got, want)
	}
	if len(got.Rates) != 2 || got.Rates["111"] != 60 || got.Rates["222"] != 30 {
		t.Fatalf("rates round-trip mismatch: got %+v", got.Rates)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	isolateConfig(t)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file should not error, got %v", err)
	}
	if got.Token != "" || got.WorkspaceID != "" || got.Currency != "" || got.Rate != 0 || got.Rates != nil {
		t.Fatalf("expected zero Config, got %+v", got)
	}
}

func TestEnvOverridesToken(t *testing.T) {
	isolateConfig(t)
	if err := Save(Config{Token: "file_tok", WorkspaceID: "1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("CLICKUP_TOKEN", "env_tok")
	got, _ := Load()
	if got.Token != "env_tok" {
		t.Fatalf("env should override token, got %q", got.Token)
	}
}

func TestValid(t *testing.T) {
	if (Config{Token: "x"}).Valid() {
		t.Fatal("missing workspace should be invalid")
	}
	if !(Config{Token: "x", WorkspaceID: "1"}).Valid() {
		t.Fatal("token+workspace should be valid")
	}
}

func TestPathUnderConfigDir(t *testing.T) {
	dir := isolateConfig(t)
	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Base(p) != "config.yml" {
		t.Fatalf("expected config.yml, got %s", p)
	}
	if !strings.HasPrefix(p, dir) {
		t.Fatalf("path %s should be under temp dir %s", p, dir)
	}
	if filepath.Base(filepath.Dir(p)) != "clup" {
		t.Fatalf("expected parent dir 'clup', got %s", filepath.Dir(p))
	}
}

// --- Task 10: clup path, legacy fallback, schema_version, env-token fix ---

func TestLoadReadsNewPath(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("token: newtok\nworkspace_id: \"1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != "newtok" {
		t.Fatalf("expected token read from new path, got %q", got.Token)
	}
}

func TestLoadFallsBackToLegacyThenMigratesOnSave(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("token: legacytok\nworkspace_id: \"1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// New path absent -> Load must fall back to the legacy file.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != "legacytok" {
		t.Fatalf("expected token read from legacy path, got %q", got.Token)
	}

	// First Save must write the new path and stub out the legacy file.
	if err := Save(got); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected new path to exist after Save: %v", err)
	}

	legacyContent, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("reading legacy file after migration: %v", err)
	}
	want := fmt.Sprintf("# moved to %s by clup\n", newPath)
	if string(legacyContent) != want {
		t.Fatalf("legacy stub mismatch: got %q want %q", legacyContent, want)
	}

	// A subsequent Load must now come from the new path, not re-trigger
	// the legacy migration dance.
	got2, err := Load()
	if err != nil {
		t.Fatalf("Load (after migration): %v", err)
	}
	if got2.Token != "legacytok" {
		t.Fatalf("expected token still readable from new path, got %q", got2.Token)
	}
}

func TestSchemaVersionAbsentIsMigratedToCurrent(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("token: tok\nworkspace_id: \"1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SchemaVersion != currentSchemaVersion {
		t.Fatalf("expected schema_version migrated to %d, got %d", currentSchemaVersion, got.SchemaVersion)
	}
}

func TestSchemaVersionFutureWarnsButLoads(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	future := currentSchemaVersion + 1
	content := fmt.Sprintf("schema_version: %d\ntoken: tok\nworkspace_id: \"1\"\n", future)
	if err := os.WriteFile(newPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load should not error on a future schema_version: %v", err)
	}
	if got.SchemaVersion != future {
		t.Fatalf("expected schema_version left untouched at %d, got %d", future, got.SchemaVersion)
	}
	if got.Token != "tok" {
		t.Fatalf("expected config to still load despite future schema_version, got token %q", got.Token)
	}
}

func TestSaveDoesNotPersistEnvToken(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := Save(Config{Token: "original_tok", WorkspaceID: "1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("CLICKUP_TOKEN", "env_tok")
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != "env_tok" {
		t.Fatalf("expected in-memory token to be env-overridden, got %q", got.Token)
	}

	if err := Save(got); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("CLICKUP_TOKEN", "") // read back without the override in play
	onDisk, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if onDisk.Token != "original_tok" {
		t.Fatalf("expected on-disk token to remain %q, got %q", "original_tok", onDisk.Token)
	}
}

func TestSaveRoundTripsTokenWithoutEnv(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := Save(Config{Token: "tok_abc", WorkspaceID: "1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != "tok_abc" {
		t.Fatalf("expected round-tripped token, got %q", got.Token)
	}

	// Steady-state stability: once schema_version has stabilized, repeated
	// Load -> Save cycles must produce byte-identical files.
	if err := Save(got); err != nil {
		t.Fatalf("Save (2nd): %v", err)
	}
	data1, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("reading config after 2nd save: %v", err)
	}

	got2, err := Load()
	if err != nil {
		t.Fatalf("Load (2nd): %v", err)
	}
	if got2.Token != "tok_abc" {
		t.Fatalf("expected token stable across round trips, got %q", got2.Token)
	}

	if err := Save(got2); err != nil {
		t.Fatalf("Save (3rd): %v", err)
	}
	data2, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("reading config after 3rd save: %v", err)
	}

	if string(data1) != string(data2) {
		t.Fatalf("expected byte-stable steady-state round trip:\ndata1=%q\ndata2=%q", data1, data2)
	}
}
