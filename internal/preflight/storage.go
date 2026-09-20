package preflight

import (
	"fmt"
	"path/filepath"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
)

// requiredCapacity is the minimum total filesystem capacity, 50 GB.
const requiredCapacity = 50_000_000_000

// Where the storage target came from.
const (
	sourceDataDir = "data-dir"
	sourceNodeDB  = "node_db"
	sourceRoot    = "root"
)

type storageTarget struct {
	path   string
	source string
}

// resolveStorageTarget picks the path whose filesystem is measured: an
// explicit --data-dir first, then the NodeDB path of a parsed configuration,
// and the root filesystem as a last resort.
func resolveStorageTarget(opts Options, config configState) storageTarget {
	if opts.DataDir != "" {
		// A relative --data-dir belongs to the working directory, not to the
		// configuration file.
		return storageTarget{path: absolute(workingDir(), opts.DataDir), source: sourceDataDir}
	}

	if config.ok && config.facts.NodeDBPath != "" {
		// A relative NodeDB path is relative to the configuration file.
		base := filepath.Dir(config.path)
		return storageTarget{path: absolute(base, config.facts.NodeDBPath), source: sourceNodeDB}
	}

	return storageTarget{path: "/", source: sourceRoot}
}

func storageTargetCheck(target storageTarget) model.Check {
	switch target.source {
	case sourceDataDir:
		return check("storage.target", model.StatusPass,
			fmt.Sprintf("using the supplied data directory %s", quotePath(target.path)),
			quotePath(target.path), "")
	case sourceNodeDB:
		return check("storage.target", model.StatusPass,
			fmt.Sprintf("using the configured NodeDB path %s", quotePath(target.path)),
			quotePath(target.path), "")
	default:
		return check("storage.target", model.StatusWarn,
			"no xrpld data path available; using root filesystem as fallback",
			quotePath(target.path), "an xrpld data path")
	}
}

func storageCapacityCheck(info probe.StorageInfo, err error) model.Check {
	expected := fmt.Sprintf(">= %s total", formatGB(requiredCapacity))
	if err != nil {
		return check("storage.capacity", model.StatusWarn,
			"filesystem capacity could not be determined", "", expected)
	}

	observed := fmt.Sprintf("%s total, %s available", formatGB(info.TotalBytes), formatGB(info.AvailableBytes))
	if info.TotalBytes >= requiredCapacity {
		return check("storage.capacity", model.StatusPass, observed, observed, expected)
	}
	return check("storage.capacity", model.StatusFail,
		fmt.Sprintf("%s, below the %s this profile requires", observed, formatGB(requiredCapacity)),
		observed, expected)
}

func storageMediaCheck(info probe.StorageInfo, err error) model.Check {
	expected := "non-rotational media"
	if err != nil {
		return check("storage.media", model.StatusWarn,
			"storage media could not be determined", probe.MediaUnknown, expected)
	}

	switch info.Media {
	case probe.MediaSSD:
		return check("storage.media", model.StatusPass,
			"kernel reports non-rotational media", info.Media, expected)
	case probe.MediaRotational:
		return check("storage.media", model.StatusFail,
			"kernel reports rotational media; xrpld needs SSD or NVMe storage", info.Media, expected)
	default:
		return check("storage.media", model.StatusWarn,
			"storage media could not be determined", probe.MediaUnknown, expected)
	}
}

// formatGB renders a byte count in decimal gigabytes, the unit the capacity
// recommendation uses.
func formatGB(bytes uint64) string {
	return fmt.Sprintf("%.1f GB", float64(bytes)/1e9)
}
