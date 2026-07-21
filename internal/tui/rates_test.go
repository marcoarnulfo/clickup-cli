package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

func TestRatesBOpensBrowser(t *testing.T) {
	m := Model{screen: screenRates, demo: true}
	m.ratesScreen = newRates(nil, config.Config{})
	u, _ := m.updateRates(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = u.(Model)
	if m.screen != screenListBrowser || m.browserScreen.origin != screenRates {
		t.Fatalf("'b' should open the browser for rates; screen=%v origin=%v", m.screen, m.browserScreen.origin)
	}
}
