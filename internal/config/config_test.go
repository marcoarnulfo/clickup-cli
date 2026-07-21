package config

import (
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
