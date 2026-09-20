// Package preflight turns local host facts and configuration facts into the
// readiness checks of the report. Probing and rendering live elsewhere: this
// package only decides.
package preflight

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/xrpld"
)

// Roles and readiness levels.
const (
	RoleNode        = "node"
	RoleValidator   = "validator"
	LevelProduction = "production"
	LevelMinimum    = "minimum"
)

// Options are the resolved command line inputs.
type Options struct {
	Role       string
	Level      string
	ConfigPath string
	DataDir    string
	Interface  string
}

// configState is what the run knows about the supplied configuration.
type configState struct {
	supplied bool
	ok       bool
	path     string
	reason   string
	facts    xrpld.ConfigFacts
}

// Run executes every applicable check and assembles the report. Checks are
// evaluated in whatever order the data requires and are always reported in
// the fixed public order.
func Run(ctx context.Context, p probe.Probe, opts Options) model.Report {
	// The configuration is read first because the storage target can come
	// from its NodeDB path.
	config := readConfig(opts)
	target := resolveStorageTarget(opts, config)

	storage, storageErr := p.Storage(target.path)
	memoryTotal, memoryErr := p.MemoryTotal()

	checks := []model.Check{
		osCheck(p),
		archCheck(p, opts.Level),
		cpuCheck(p, opts.Level),
		memoryCheck(memoryTotal, memoryErr, opts.Level),

		storageTargetCheck(target),
		storageCapacityCheck(storage, storageErr),
		storageMediaCheck(storage, storageErr),

		networkCheck(p, opts),
		timeCheck(ctx, p, opts.Level),
		nofileCheck(ctx, p, opts.Level),

		binaryCheck(ctx, p),

		configFileCheck(config),
		nodeSizeCheck(config, opts.Level, memoryTotal, memoryErr),
		nodeDBCheck(config),
		peerPortCheck(config),
	}

	if opts.Role == RoleValidator {
		checks = append(checks,
			validatorConfigCheck(config),
			validatorCredentialsCheck(config),
			validatorLegacySeedCheck(config),
			validatorConfigModeCheck(config),
			validatorAPIBindCheck(config),
		)
	}

	return model.NewReport(opts.Role, opts.Level, checks)
}

func readConfig(opts Options) configState {
	if opts.ConfigPath == "" {
		return configState{}
	}

	state := configState{supplied: true, path: opts.ConfigPath}
	facts, err := xrpld.ReadFile(opts.ConfigPath)
	if err != nil {
		state.reason = sanitize(err.Error())
		return state
	}
	state.ok = true
	state.facts = facts
	return state
}

// check builds one check result.
func check(id string, status model.Status, summary, observed, expected string) model.Check {
	return model.Check{
		ID:       id,
		Status:   status,
		Summary:  summary,
		Observed: observed,
		Expected: expected,
	}
}

func skipped(id, summary string) model.Check {
	return model.Check{ID: id, Status: model.StatusSkip, Summary: summary}
}

// worst returns the more severe of two statuses, where skip is the mildest
// and fail the most severe.
func worst(a, b model.Status) model.Status {
	rank := map[model.Status]int{
		model.StatusSkip: 0,
		model.StatusPass: 1,
		model.StatusWarn: 2,
		model.StatusFail: 3,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// quotePath renders a path from configuration or the command line so that it
// cannot inject control characters or extra lines into a report.
func quotePath(path string) string {
	return strconv.Quote(sanitize(path))
}

// sanitize strips control characters from externally supplied text.
func sanitize(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		if r < 0x20 || r == 0x7f {
			builder.WriteRune(' ')
			continue
		}
		builder.WriteRune(r)
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// workingDir returns the current directory, falling back to the root when it
// cannot be determined.
func workingDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "/"
}

// absolute resolves path against base when it is relative.
func absolute(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}
