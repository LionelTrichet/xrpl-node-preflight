package preflight

import (
	"context"
	"fmt"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
)

// gibibyte is the unit used for memory thresholds.
const gibibyte = 1024 * 1024 * 1024

// Host thresholds of the two readiness profiles.
const (
	productionCPUs   = 8
	minimumCPUs      = 4
	productionMemory = 64 * gibibyte
	minimumMemory    = 16 * gibibyte
)

func osCheck(p probe.Probe) model.Check {
	if p.OS() == "linux" {
		return check("host.os", model.StatusPass, "Linux", p.OS(), "linux")
	}
	return check("host.os", model.StatusFail,
		"xrpld requires Linux", p.OS(), "linux")
}

func archCheck(p probe.Probe, level string) model.Check {
	arch := p.Arch()
	switch {
	case arch == "amd64":
		return check("host.arch", model.StatusPass, "amd64", arch, "amd64")
	case arch == "arm64" && level == LevelMinimum:
		return check("host.arch", model.StatusWarn,
			"arm64 is outside the recommended 64-bit x86 architecture", arch, "amd64")
	default:
		return check("host.arch", model.StatusFail,
			"unsupported architecture for an XRP Ledger node", arch, "amd64")
	}
}

func cpuCheck(p probe.Probe, level string) model.Check {
	required := productionCPUs
	if level == LevelMinimum {
		required = minimumCPUs
	}

	count, err := p.CPUCount()
	if err != nil {
		return check("host.cpu", model.StatusWarn,
			"logical CPU count could not be determined", "", fmt.Sprintf(">= %d logical CPUs", required))
	}

	observed := fmt.Sprintf("%d logical CPUs", count)
	expected := fmt.Sprintf(">= %d logical CPUs", required)
	if count >= required {
		return check("host.cpu", model.StatusPass, observed, observed, expected)
	}
	return check("host.cpu", model.StatusFail,
		fmt.Sprintf("%s, below the %d logical CPUs this profile requires", observed, required),
		observed, expected)
}

func memoryCheck(total uint64, err error, level string) model.Check {
	required := uint64(productionMemory)
	if level == LevelMinimum {
		required = minimumMemory
	}
	expected := fmt.Sprintf(">= %d GiB", required/gibibyte)

	if err != nil {
		return check("host.memory", model.StatusWarn,
			"total memory could not be determined", "", expected)
	}

	observed := formatGiB(total)
	if total >= required {
		return check("host.memory", model.StatusPass,
			fmt.Sprintf("%s total memory", observed), observed, expected)
	}
	return check("host.memory", model.StatusFail,
		fmt.Sprintf("%s total memory, below the %d GiB this profile requires", observed, required/gibibyte),
		observed, expected)
}

func binaryCheck(ctx context.Context, p probe.Probe) model.Check {
	info, err := p.XRPLDVersion(ctx)
	switch {
	case err != nil:
		return check("xrpld.binary", model.StatusWarn,
			"xrpld version could not be determined", "", "xrpld --version")
	case !info.Found:
		return check("xrpld.binary", model.StatusWarn,
			"xrpld not found in PATH", "", "xrpld in PATH")
	case !info.VersionOK || info.Version == "":
		return check("xrpld.binary", model.StatusWarn,
			"xrpld found but did not report a version", sanitize(info.Path), "xrpld --version")
	default:
		version := sanitize(info.Version)
		return check("xrpld.binary", model.StatusPass, version, version, "")
	}
}

func formatGiB(bytes uint64) string {
	return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(gibibyte))
}
