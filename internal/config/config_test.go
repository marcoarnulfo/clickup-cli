package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfig reindirizza la config in una dir temporanea su TUTTE le
// piattaforme. Su macOS os.UserConfigDir usa $HOME/Library/Application Support
// (ignora XDG_CONFIG_HOME); su Linux usa XDG_CONFIG_HOME. Settiamo entrambe,
// così i test non toccano MAI la config reale dell'utente.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CLICKUP_TOKEN", "") // evita override durante i test
	return dir
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
}
