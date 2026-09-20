// Package report renders a report as text or JSON. Rendering never inspects
// the host: it only formats what the checks produced.
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
)

// WriteJSON writes the report as indented JSON with a trailing newline.
func WriteJSON(w io.Writer, report model.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	return nil
}
