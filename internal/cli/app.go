// Package cli parses the command line and wires the probe, the checks and
// the renderer together. It contains no readiness logic of its own.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/preflight"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/report"
)

const usage = "usage: xrpl-node-preflight check [--role node|validator] [--level production|minimum] " +
	"[--config PATH] [--data-dir PATH] [--interface NAME] [--format text|json]"

// Output formats.
const (
	formatText = "text"
	formatJSON = "json"
)

// Run executes one invocation and returns the exit code: 0 when no check
// failed, 1 when at least one failed, and 2 for a usage or tool error.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, probe.NewLinux())
}

// run is Run with an explicit probe, which lets the tests exercise the whole
// command line without depending on the host they run on.
func run(args []string, stdout, stderr io.Writer, p probe.Probe) int {
	if len(args) == 0 {
		return usageError(stderr, "missing command")
	}
	if args[0] != "check" {
		return usageError(stderr, fmt.Sprintf("unknown command %q", args[0]))
	}

	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	role := flags.String("role", preflight.RoleNode, "node or validator")
	level := flags.String("level", preflight.LevelProduction, "production or minimum")
	configPath := flags.String("config", "", "path to xrpld.cfg")
	dataDir := flags.String("data-dir", "", "path to the xrpld data directory")
	iface := flags.String("interface", "", "network interface name")
	format := flags.String("format", formatText, "text or json")

	if err := flags.Parse(args[1:]); err != nil {
		return usageError(stderr, err.Error())
	}
	if flags.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("unexpected argument %q", flags.Arg(0)))
	}

	if *role != preflight.RoleNode && *role != preflight.RoleValidator {
		return usageError(stderr, fmt.Sprintf("invalid role %q", *role))
	}
	if *level != preflight.LevelProduction && *level != preflight.LevelMinimum {
		return usageError(stderr, fmt.Sprintf("invalid level %q", *level))
	}
	if *format != formatText && *format != formatJSON {
		return usageError(stderr, fmt.Sprintf("invalid format %q", *format))
	}
	for name, value := range map[string]string{"config": *configPath, "data-dir": *dataDir, "interface": *iface} {
		if flagSet(flags, name) && value == "" {
			return usageError(stderr, fmt.Sprintf("flag --%s requires a value", name))
		}
	}

	options := preflight.Options{
		Role:       *role,
		Level:      *level,
		ConfigPath: *configPath,
		DataDir:    *dataDir,
		Interface:  *iface,
	}

	result := preflight.Run(context.Background(), p, options)
	return write(stdout, stderr, result, *format)
}

func write(stdout, stderr io.Writer, result model.Report, format string) int {
	var err error
	if format == formatJSON {
		err = report.WriteJSON(stdout, result)
	} else {
		err = report.WriteText(stdout, result)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	if result.Summary.Fail > 0 {
		return 1
	}
	return 0
}

// flagSet reports whether the flag was given on the command line.
func flagSet(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func usageError(stderr io.Writer, message string) int {
	fmt.Fprintf(stderr, "xrpl-node-preflight: %s\n%s\n", message, usage)
	return 2
}
