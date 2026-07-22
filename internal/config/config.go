// Package config manages reading/writing the user configuration.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// currentSchemaVersion is the schema_version this binary understands and
// stamps on any config missing it.
const currentSchemaVersion = 1

// Config is the persisted CLI configuration.
type Config struct {
	SchemaVersion int                `yaml:"schema_version"`
	Token         string             `yaml:"token"`
	WorkspaceID   string             `yaml:"workspace_id"`
	Currency      string             `yaml:"currency"`
	Rate          float64            `yaml:"rate"`
	Rates         map[string]float64 `yaml:"rates,omitempty"` // list_id -> rate override

	// fileToken is the token as it was read from disk, before any
	// CLICKUP_TOKEN env override is applied. yaml.v3 never marshals
	// unexported fields, so this never leaks into the config file. Save
	// uses it to avoid persisting an env-provided token to disk.
	fileToken string
	// loadedFromLegacy records whether Load fell back to the legacy
	// pre-rebrand path. Save uses it to decide whether the legacy file
	// should be rewritten to a pointer stub.
	loadedFromLegacy bool
}

// Valid reports whether the config can be used to query the API.
func (c Config) Valid() bool {
	return c.Token != "" && c.WorkspaceID != ""
}

// configPath and legacyConfigPath are injectable so tests can point them at
// a t.TempDir() without touching the real user config. They default to
// os.UserConfigDir()/clup/config.yml (macOS: "~/Library/Application
// Support/clup", Linux: "~/.config/clup") and the pre-rebrand
// os.UserConfigDir()/clickup-cli/config.yml, respectively.
var (
	configPath = func() (string, error) {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "clup", "config.yml"), nil
	}
	legacyConfigPath = func() (string, error) {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "clickup-cli", "config.yml"), nil
	}
)

// Path returns the config file path, e.g. os.UserConfigDir()/clup/config.yml
// (macOS: "~/Library/Application Support/clup/config.yml", Linux:
// "~/.config/clup/config.yml").
func Path() (string, error) {
	return configPath()
}

// Load reads the config from disk. It tries the current path first; if that
// file does not exist, it falls back to the legacy pre-rebrand path and
// records the fallback so a subsequent Save can migrate it. Missing files in
// both locations -> Config{} with no error. The CLICKUP_TOKEN env var, if
// set, overrides the token from the file (but Save never persists that
// override back to disk).
func Load() (Config, error) {
	var c Config

	p, err := configPath()
	if err != nil {
		return c, err
	}

	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		lp, err := legacyConfigPath()
		if err != nil {
			return c, err
		}
		legacyData, err := os.ReadFile(lp)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// no config anywhere: empty config
		case err != nil:
			return c, err
		default:
			if err := yaml.Unmarshal(legacyData, &c); err != nil {
				return c, err
			}
			c.loadedFromLegacy = true
		}
	case err != nil:
		return c, err
	default:
		if err := yaml.Unmarshal(data, &c); err != nil {
			return c, err
		}
	}

	c.fileToken = c.Token

	if env := os.Getenv("CLICKUP_TOKEN"); env != "" {
		c.Token = env
	}

	c = migrate(c)

	return c, nil
}

// migrate stamps schema_version on the in-memory config. It is idempotent:
// an absent (zero) version is stamped to currentSchemaVersion. A version
// newer than currentSchemaVersion means a future binary wrote this file;
// Load warns on stderr but still loads the config as-is (Load may warn,
// Save must stay silent). There is no migration registry: this is a single
// stamp, not a multi-step upgrade path.
func migrate(c Config) Config {
	switch {
	case c.SchemaVersion == 0:
		c.SchemaVersion = currentSchemaVersion
	case c.SchemaVersion > currentSchemaVersion:
		fmt.Fprintf(os.Stderr, "warning: config schema_version %d is newer than the version this build understands (%d); loading anyway\n", c.SchemaVersion, currentSchemaVersion)
	}
	return c
}

// Save writes the config to disk, creating the necessary directories. It
// never persists a CLICKUP_TOKEN env override: if the config's token equals
// the current env value, the on-disk token is left as whatever was
// originally read from the file. If the config was loaded from the legacy
// path and this is the first write to the new path, the legacy file is
// rewritten to a pointer stub. Save is silent (no stdout/stderr output).
func Save(c Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}

	out := c
	if env := os.Getenv("CLICKUP_TOKEN"); env != "" && c.Token == env {
		out.Token = c.fileToken
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	_, statErr := os.Stat(p)
	newPathExisted := statErr == nil

	data, err := yaml.Marshal(out)
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}

	if c.loadedFromLegacy && !newPathExisted {
		lp, err := legacyConfigPath()
		if err != nil {
			return err
		}
		stub := fmt.Sprintf("# moved to %s by clup\n", p)
		if err := os.WriteFile(lp, []byte(stub), 0o600); err != nil {
			return err
		}
	}

	return nil
}
