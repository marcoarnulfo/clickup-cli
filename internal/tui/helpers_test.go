package tui

import (
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
)

// testModel builds a Model the way production does, with the built-in palette
// and bindings. It exists so that growing New's signature touches one line
// instead of forty-three.
func testModel(cfg config.Config) Model {
	return New(cfg, themes.Default(), DefaultKeyTable())
}
