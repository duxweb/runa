package console

import "context"

// Summary describes one framework/runtime module row for the console overview.
type Summary struct {
	Module  string `json:"module"`
	Summary string `json:"summary"`
	Default string `json:"default"`
}

// SummaryResolver resolves console summary rows.
type SummaryResolver func(context.Context, AppContext) []Summary
