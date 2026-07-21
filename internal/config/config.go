// Package config manages reading/writing the user configuration.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the persisted CLI configuration.
type Config struct {
	Token       string             `yaml:"token"`
	WorkspaceID string             `yaml:"workspace_id"`
	Currency    string             `yaml:"currency"`
	Rate        float64            `yaml:"rate"`
	Rates       map[string]float64 `yaml:"rates,omitempty"` // list_id -> rate override
}

// Valid reports whether the config can be used to query the API.
func (c Config) Valid() bool {
	return c.Token != "" && c.WorkspaceID != ""
}

// Path returns the config file path, e.g. ~/.config/clickup-cli/config.yml.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "clickup-cli", "config.yml"), nil
}

// Load reads the config from disk. Missing file -> Config{} with no error.
// The CLICKUP_TOKEN env var, if set, overrides the token from the file.
func Load() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// no file: empty config
	case err != nil:
		return c, err
	default:
		if err := yaml.Unmarshal(data, &c); err != nil {
			return c, err
		}
	}
	if env := os.Getenv("CLICKUP_TOKEN"); env != "" {
		c.Token = env
	}
	return c, nil
}

// Save writes the config to disk, creating the necessary directories.
func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
