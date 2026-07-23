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
// stamps on any config below it.
const currentSchemaVersion = 2

// Override is a (list, member) pair rate override: the most specific entry
// in the billing rate precedence.
type Override struct {
	List   string  `yaml:"list"`
	Member int     `yaml:"member"`
	Rate   float64 `yaml:"rate"`
}

// Rounding configures how billable hours are rounded before invoicing.
// Increment is a human duration string (e.g. "15m"); parsing it is not this
// package's job.
type Rounding struct {
	Increment string `yaml:"increment"`
	Mode      string `yaml:"mode"`
	Scope     string `yaml:"scope"`
}

// Billing carries the v2 billing configuration: per-member rates, (list,
// member) overrides, per-list currencies, budgets and a rounding rule. It is
// additive to the pre-existing top-level Rate/Rates/Currency fields, which
// stay in place unchanged.
type Billing struct {
	DefaultCurrency string             `yaml:"default_currency,omitempty"`
	RatesByMember   map[int]float64    `yaml:"rates_by_member,omitempty"`
	RateOverrides   []Override         `yaml:"rate_overrides,omitempty"`
	Currencies      map[string]string  `yaml:"currencies,omitempty"` // listID -> currency ISO
	Budgets         map[string]float64 `yaml:"budgets,omitempty"`
	Rounding        Rounding           `yaml:"rounding,omitempty"`
}

// Config is the persisted CLI configuration.
type Config struct {
	SchemaVersion int                `yaml:"schema_version"`
	Token         string             `yaml:"token"`
	WorkspaceID   string             `yaml:"workspace_id"`
	Currency      string             `yaml:"currency"`
	Rate          float64            `yaml:"rate"`
	Rates         map[string]float64 `yaml:"rates,omitempty"` // list_id -> rate override
	Timezone      string             `yaml:"timezone,omitempty"`
	Billing       Billing            `yaml:"billing,omitempty"`

	// fileToken is the token as it was read from disk, before any
	// CLICKUP_TOKEN env override is applied. yaml.v3 never marshals
	// unexported fields, so this never leaks into the config file. Save
	// uses it to avoid persisting an env-provided token to disk.
	fileToken string
	// tokenFromEnv records whether THIS Config's Token was populated by the
	// CLICKUP_TOKEN env override during Load (true only when Load actually
	// applied it). This is a provenance flag, not a value comparison: a
	// freshly-constructed Config{} (e.g. from the TUI setup wizard) always
	// has tokenFromEnv == false, so Save persists its Token verbatim even if
	// it happens to equal the current env value. Only a Load-ed config whose
	// token really came from the env has this set, which is what makes Save
	// substitute fileToken instead of writing the env secret to disk.
	tokenFromEnv bool
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
		c.tokenFromEnv = true
	}

	c = migrate(c)

	return c, nil
}

// migrate stamps schema_version on the in-memory config. It is idempotent:
// any version below currentSchemaVersion (including an absent/zero version)
// is stamped up to currentSchemaVersion. The migration is purely additive --
// no field is moved, renamed or defaulted by this function; a v1 file's
// existing rate/rates/currency values are left exactly as read, and the new
// v2 fields (Timezone, Billing) simply stay at their zero value until the
// user sets them. A version newer than currentSchemaVersion means a future
// binary wrote this file; Load warns on stderr but still loads the config
// as-is (Load may warn, Save must stay silent). There is no migration
// registry: this is a single stamp, not a multi-step upgrade path.
func migrate(c Config) Config {
	switch {
	case c.SchemaVersion < currentSchemaVersion:
		c.SchemaVersion = currentSchemaVersion
	case c.SchemaVersion > currentSchemaVersion:
		fmt.Fprintf(os.Stderr, "warning: config schema_version %d is newer than the version this build understands (%d); loading anyway\n", c.SchemaVersion, currentSchemaVersion)
	}
	return c
}

// Save writes the config to disk, creating the necessary directories. It
// never persists a CLICKUP_TOKEN env override: if this Config's Token was
// populated by the env override during Load (tokenFromEnv == true), the
// on-disk token is written as whatever was originally read from the file
// instead. This is a provenance check, not a value comparison, so it does not
// misfire on a freshly-constructed Config (e.g. from the TUI setup wizard)
// whose typed token happens to equal the env value. If the config was loaded
// from the legacy path and this is the first write to the new path, the
// legacy file is rewritten to a pointer stub. Save is silent (no
// stdout/stderr output).
func Save(c Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}

	out := c
	if c.tokenFromEnv {
		out.Token = c.fileToken
	}
	if out.SchemaVersion == 0 {
		out.SchemaVersion = currentSchemaVersion
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
