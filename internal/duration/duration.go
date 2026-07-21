// Package duration parsa durate in formato umano (2h30, 1.5h, 90m, 45).
// È puro: nessun I/O, nessuna dipendenza esterna.
package duration

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reHM  = regexp.MustCompile(`^(\d+)h(\d+)m?$`)    // 2h30, 2h30m
	reH   = regexp.MustCompile(`^(\d+(?:\.\d+)?)h$`) // 2h, 1.5h
	reM   = regexp.MustCompile(`^(\d+)m$`)           // 90m
	reNum = regexp.MustCompile(`^\d+(?:\.\d+)?$`)    // 45 (numero nudo = ore)
)

// Parse converte una stringa durata in time.Duration.
// Formati: Nh, NhMm/NhM (ore+minuti), N.Nh/N,Nh (ore decimali), Nm (minuti),
// numero nudo = ore. La virgola è accettata come il punto (tastiera italiana).
// Ritorna errore su input non riconosciuto o durata ≤ 0.
func Parse(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0, fmt.Errorf("durata vuota")
	}

	var d time.Duration
	switch {
	case reHM.MatchString(s):
		m := reHM.FindStringSubmatch(s)
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		d = time.Duration(h)*time.Hour + time.Duration(min)*time.Minute
	case reH.MatchString(s):
		m := reH.FindStringSubmatch(s)
		f, _ := strconv.ParseFloat(m[1], 64)
		d = time.Duration(f * float64(time.Hour))
	case reM.MatchString(s):
		m := reM.FindStringSubmatch(s)
		min, _ := strconv.Atoi(m[1])
		d = time.Duration(min) * time.Minute
	case reNum.MatchString(s):
		f, _ := strconv.ParseFloat(s, 64)
		d = time.Duration(f * float64(time.Hour))
	default:
		return 0, fmt.Errorf("durata non riconosciuta: %q", s)
	}

	if d <= 0 {
		return 0, fmt.Errorf("durata deve essere > 0: %q", s)
	}
	return d, nil
}
