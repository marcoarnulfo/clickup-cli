package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

// TestSaveFreshConfigWritesTypedTokenEvenWhenEqualToEnv guards against a
// silent token wipe: the TUI setup wizard builds a fresh Config{} (never
// through Load) and calls Save directly. If the guard in Save compares
// Token by VALUE against os.Getenv("CLICKUP_TOKEN"), it misfires whenever the
// user happens to type the same token as an exported CLICKUP_TOKEN, wiping
// the on-disk token to "" (fileToken is zero-value on a fresh Config). The
// fix is a provenance flag (tokenFromEnv) that only Load can set, so a fresh
// Config always persists its Token verbatim.
func TestSaveFreshConfigWritesTypedTokenEvenWhenEqualToEnv(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	t.Setenv("CLICKUP_TOKEN", "pk_real")

	fresh := Config{Token: "pk_real", WorkspaceID: "900"}
	if err := Save(fresh); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("CLICKUP_TOKEN", "") // read back without the override in play
	onDisk, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if onDisk.Token != "pk_real" {
		t.Fatalf("expected on-disk token to be the typed value %q, got %q (silent token wipe)", "pk_real", onDisk.Token)
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

// --- Task 6: schema v2 (timezone + billing block) ---

func TestLoadV1MigratesToV2(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// A v1 config: schema_version 1, only the old rate/rates/currency fields.
	v1 := "schema_version: 1\n" +
		"token: tok\n" +
		"workspace_id: \"1\"\n" +
		"currency: EUR\n" +
		"rate: 45\n" +
		"rates:\n" +
		"  \"111\": 60\n"
	if err := os.WriteFile(newPath, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SchemaVersion != 2 {
		t.Fatalf("expected schema_version migrated to 2, got %d", got.SchemaVersion)
	}
	if got.Currency != "EUR" || got.Rate != 45 {
		t.Fatalf("expected v1 fields preserved, got currency=%q rate=%v", got.Currency, got.Rate)
	}
	if len(got.Rates) != 1 || got.Rates["111"] != 60 {
		t.Fatalf("expected v1 rates preserved, got %+v", got.Rates)
	}
	if got.Timezone != "" {
		t.Fatalf("expected Timezone to stay empty on migration, got %q", got.Timezone)
	}
	if !reflect.DeepEqual(got.Billing, Billing{}) {
		t.Fatalf("expected Billing to stay zero on migration, got %+v", got.Billing)
	}
}

func TestSaveLoadV2RoundTrip(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	want := Config{
		Token:       "tok_v2",
		WorkspaceID: "900",
		Currency:    "EUR",
		Rate:        45,
		Timezone:    "Europe/Rome",
		Billing: Billing{
			DefaultCurrency: "EUR",
			RatesByMember:   map[int]float64{1: 50, 2: 65},
			RateOverrides: []Override{
				{List: "111", Member: 1, Rate: 80},
			},
			Currencies: map[string]string{"111": "USD"},
			Budgets:    map[string]float64{"111": 1000},
			Rounding:   Rounding{Increment: "15m", Mode: "up", Scope: "entry"},
		},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Timezone != want.Timezone {
		t.Fatalf("timezone round-trip mismatch: got %q want %q", got.Timezone, want.Timezone)
	}
	if !reflect.DeepEqual(got.Billing, want.Billing) {
		t.Fatalf("billing round-trip mismatch:\ngot  %+v\nwant %+v", got.Billing, want.Billing)
	}
	if got.SchemaVersion != 2 {
		t.Fatalf("expected schema_version 2, got %d", got.SchemaVersion)
	}
}

// TestMigrateIdempotentOnV2 guards against re-migration drift (amendment M3):
// loading an already-v2 config, saving it, and loading it again must leave
// the whole Config unchanged -- no field drift, no repeated migration side
// effects.
func TestMigrateIdempotentOnV2(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "clup", "config.yml")
	legacyPath := filepath.Join(dir, "clickup-cli", "config.yml")
	withPaths(t, newPath, legacyPath)

	seed := Config{
		Token:       "tok_v2",
		WorkspaceID: "900",
		Currency:    "EUR",
		Rate:        45,
		Rates:       map[string]float64{"111": 60},
		Timezone:    "Europe/Rome",
		Billing: Billing{
			DefaultCurrency: "EUR",
			RatesByMember:   map[int]float64{1: 50},
			RateOverrides:   []Override{{List: "111", Member: 1, Rate: 80}},
			Currencies:      map[string]string{"111": "USD"},
			Budgets:         map[string]float64{"111": 1000},
			Rounding:        Rounding{Increment: "15m", Mode: "up", Scope: "entry"},
		},
	}
	if err := Save(seed); err != nil {
		t.Fatalf("Save: %v", err)
	}

	first, err := Load()
	if err != nil {
		t.Fatalf("Load (1st): %v", err)
	}
	if first.SchemaVersion != 2 {
		t.Fatalf("expected schema_version 2 after first load, got %d", first.SchemaVersion)
	}

	if err := Save(first); err != nil {
		t.Fatalf("Save (2nd): %v", err)
	}
	second, err := Load()
	if err != nil {
		t.Fatalf("Load (2nd): %v", err)
	}

	// Clear the unexported provenance fields before comparing: they are not
	// part of the persisted schema and their transient state (e.g.
	// loadedFromLegacy) is irrelevant to this test's claim.
	first.fileToken, second.fileToken = "", ""
	first.tokenFromEnv, second.tokenFromEnv = false, false
	first.loadedFromLegacy, second.loadedFromLegacy = false, false

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected idempotent migrate/save/load cycle, no drift:\nfirst  %+v\nsecond %+v", first, second)
	}
}
