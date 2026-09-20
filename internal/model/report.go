// Package model holds the public report contract: the statuses, the checks
// and the overall result. Check IDs and the JSON shape are part of the
// machine-readable interface of the tool.
package model

// Status is the verdict of a single check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

// Check is one readiness check. Observed and Expected are optional context
// for the operator; the summary is always present.
type Check struct {
	ID       string `json:"id"`
	Status   Status `json:"status"`
	Summary  string `json:"summary"`
	Observed string `json:"observed,omitempty"`
	Expected string `json:"expected,omitempty"`
}

// Summary counts the checks by status.
type Summary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// Report is the full result of one run. It deliberately carries no timestamp,
// host name or identifier, so equivalent facts always produce equivalent
// output.
type Report struct {
	SchemaVersion int     `json:"schemaVersion"`
	Role          string  `json:"role"`
	Level         string  `json:"level"`
	Result        string  `json:"result"`
	Checks        []Check `json:"checks"`
	Summary       Summary `json:"summary"`
}

// Overall results.
const (
	ResultReady             = "ready"
	ResultReadyWithWarnings = "ready_with_warnings"
	ResultNotReady          = "not_ready"
)

// SchemaVersion is the version of the report contract.
const SchemaVersion = 1

// NewReport counts the checks and derives the overall result: any failure
// makes the host not ready, warnings alone downgrade it to ready with
// warnings, and skipped checks never downgrade anything.
func NewReport(role, level string, checks []Check) Report {
	summary := Summary{}
	for _, check := range checks {
		switch check.Status {
		case StatusPass:
			summary.Pass++
		case StatusWarn:
			summary.Warn++
		case StatusFail:
			summary.Fail++
		case StatusSkip:
			summary.Skip++
		}
	}

	result := ResultReady
	switch {
	case summary.Fail > 0:
		result = ResultNotReady
	case summary.Warn > 0:
		result = ResultReadyWithWarnings
	}

	return Report{
		SchemaVersion: SchemaVersion,
		Role:          role,
		Level:         level,
		Result:        result,
		Checks:        checks,
		Summary:       summary,
	}
}
