// Package report contiene il modello dominio e la logica di aggregazione
// delle ore. È puro: nessun I/O, nessuna dipendenza esterna.
package report

import "time"

// TimeEntry è una singola voce di tempo normalizzata dal dominio ClickUp.
type TimeEntry struct {
	ID       string
	TaskID   string
	TaskName string
	ListID   string
	ListName string // il "progetto"
	UserID   int
	UserName string
	Start    time.Time
	Duration time.Duration
}

// Bucket è una riga aggregata del report. I tag JSON servono all'export
// (il package resta puro: i tag non introducono dipendenze).
type Bucket struct {
	Label  string  `json:"label"`
	Hours  float64 `json:"hours"`
	Amount float64 `json:"amount"`
}

// Report è il risultato aggregato pronto per la presentazione/export.
type Report struct {
	Year        int
	Month       time.Month
	Scope       string // "me" | "team"
	GroupBy     string
	Currency    string
	Rate        float64
	Buckets     []Bucket
	TotalHours  float64
	TotalAmount float64
}
