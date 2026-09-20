package probe

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

// maxCommandOutput bounds how much child output is retained, 64 KiB.
const maxCommandOutput = 65536

// runCommand executes a fixed local command with a timeout and bounded
// output. Executable names and arguments are always literals in this
// package: nothing from the configuration or the command line selects a
// program, and no shell is involved.
func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(ctx, name, args...)
	// A predictable locale keeps the output parseable.
	command.Env = append(command.Environ(), "LC_ALL=C")

	output := newCappedWriter(maxCommandOutput)
	command.Stdout = output
	command.Stderr = output

	err := command.Run()
	text := output.String()
	if err != nil {
		return text, fmt.Errorf("%s: %w", name, err)
	}
	return text, nil
}

// firstDisplayLine reduces child output to one sanitized line that is safe to
// put in a report: no control characters, no escape sequences, no newlines.
func firstDisplayLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if sanitized := sanitizeDisplay(line); sanitized != "" {
			return sanitized
		}
	}
	return ""
}

// sanitizeDisplay removes control characters and collapses whitespace so that
// externally produced text cannot inject terminal sequences or extra report
// lines.
func sanitizeDisplay(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		switch {
		case r == ' ':
			builder.WriteRune(' ')
		case unicode.IsControl(r):
			// Dropped: escape sequences must never reach the report.
			builder.WriteRune(' ')
		case !unicode.IsPrint(r):
			builder.WriteRune('?')
		default:
			builder.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}
