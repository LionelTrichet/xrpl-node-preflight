package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
)

// idColumn is the fixed width of the check-identifier column. Layout never
// depends on the terminal: the output must look the same in a CI log.
const idColumn = 28

// WriteText writes the human-readable report. There are no colors, symbols
// or escape sequences in the output.
func WriteText(w io.Writer, report model.Report) error {
	var builder strings.Builder

	builder.WriteString("XRPL Node Preflight\n\n")
	fmt.Fprintf(&builder, "Role:  %s\n", report.Role)
	fmt.Fprintf(&builder, "Level: %s\n\n", report.Level)

	for _, check := range report.Checks {
		fmt.Fprintf(&builder, "%-4s  %-*s%s\n",
			strings.ToUpper(string(check.Status)), idColumn, check.ID, check.Summary)
	}

	fmt.Fprintf(&builder, "\nResult: %s\n", resultText(report.Result))
	fmt.Fprintf(&builder, "%d checks: %d pass, %d warn, %d fail, %d skip\n",
		len(report.Checks), report.Summary.Pass, report.Summary.Warn,
		report.Summary.Fail, report.Summary.Skip)

	if _, err := io.WriteString(w, builder.String()); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}

func resultText(result string) string {
	switch result {
	case model.ResultReady:
		return "READY"
	case model.ResultReadyWithWarnings:
		return "READY WITH WARNINGS"
	default:
		return "NOT READY"
	}
}
