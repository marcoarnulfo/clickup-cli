// Package config gestisce la lettura/scrittura della configurazione utente.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config è la configurazione persistita della CLI.
type Config struct {
	Token       string             `yaml:"token"`
	WorkspaceID string             `yaml:"workspace_id"`
	Currency    string             `yaml:"currency"`
	Rate        float64            `yaml:"rate"`
	Rates       map[string]float64 `yaml:"rates,omitempty"` // list_id -> tariffa override
}

// Valid indica se la config è utilizzabile per interrogare l'API.
func (c Config) Valid() bool {
	return c.Token != "" && c.WorkspaceID != ""
}

// Path ritorna il percorso del file di config, es. ~/.config/clickup-cli/config.yml.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "clickup-cli", "config.yml"), nil
}

// Load legge la config dal disco. File mancante -> Config{} senza errore.
// L'env CLICKUP_TOKEN, se valorizzato, sovrascrive il token del file.
func Load() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// nessun file: config vuota
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

// Save scrive la config su disco creando le directory necessarie.
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
