package preflight

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/xrpld"
)

// Peer ports with a recognized meaning. Any valid port is accepted.
const (
	ianaPeerPort   = "2459"
	legacyPeerPort = "51235"
)

// minimumHugeMemory is the memory below which node_size huge is questionable
// on the minimum profile.
const minimumHugeMemory = 32 * gibibyte

func configFileCheck(config configState) model.Check {
	switch {
	case !config.supplied:
		return skipped("config.file", "no configuration file supplied")
	case !config.ok:
		return check("config.file", model.StatusFail,
			fmt.Sprintf("configuration could not be read: %s", config.reason),
			quotePath(config.path), "a readable xrpld.cfg")
	default:
		return check("config.file", model.StatusPass,
			fmt.Sprintf("configuration read from %s", quotePath(config.path)),
			quotePath(config.path), "")
	}
}

func nodeSizeCheck(config configState, level string, memoryTotal uint64, memoryErr error) model.Check {
	if !config.ok {
		return skipped("config.node_size", configSkipReason(config))
	}

	value := strings.TrimSpace(config.facts.NodeSize)
	if value == "" {
		return check("config.node_size", model.StatusPass,
			"node_size omitted; xrpld can select a value automatically", "", "")
	}

	normalized := strings.ToLower(value)
	switch normalized {
	case "tiny", "small", "medium", "large", "huge":
	default:
		return check("config.node_size", model.StatusFail,
			fmt.Sprintf("node_size %s is not a valid value", quotePath(value)),
			sanitize(value), "tiny, small, medium, large or huge")
	}

	if level == LevelMinimum {
		switch normalized {
		case "large":
			return check("config.node_size", model.StatusWarn,
				"node_size large is discouraged by current capacity guidance", normalized, "")
		case "huge":
			if memoryErr == nil && memoryTotal < minimumHugeMemory {
				return check("config.node_size", model.StatusWarn,
					fmt.Sprintf("node_size huge with %s of memory", formatGiB(memoryTotal)),
					normalized, ">= 32 GiB for huge")
			}
		}
		return check("config.node_size", model.StatusPass,
			fmt.Sprintf("node_size %s", normalized), normalized, "")
	}

	switch normalized {
	case "huge":
		return check("config.node_size", model.StatusPass, "node_size huge", normalized, "huge")
	case "large":
		return check("config.node_size", model.StatusWarn,
			"node_size large is discouraged by current capacity guidance, which recommends huge for production",
			normalized, "huge")
	default:
		return check("config.node_size", model.StatusWarn,
			fmt.Sprintf("node_size %s is below the production recommendation", normalized),
			normalized, "huge")
	}
}

func nodeDBCheck(config configState) model.Check {
	if !config.ok {
		return skipped("config.node_db", configSkipReason(config))
	}

	facts := config.facts
	if facts.NodeDBType == "" || facts.NodeDBPath == "" {
		return check("config.node_db", model.StatusFail,
			"[node_db] must define both type and path", "", "type and path")
	}

	path := absolute(filepath.Dir(config.path), facts.NodeDBPath)
	switch strings.ToLower(facts.NodeDBType) {
	case "nudb":
		return check("config.node_db", model.StatusPass,
			fmt.Sprintf("NuDB at %s", quotePath(path)), "NuDB", "NuDB")
	case "rocksdb":
		return check("config.node_db", model.StatusWarn,
			"RocksDB is a legacy NodeDB backend; current capacity guidance recommends NuDB for most production deployments",
			"RocksDB", "NuDB")
	default:
		return check("config.node_db", model.StatusFail,
			fmt.Sprintf("unknown NodeDB type %s", quotePath(facts.NodeDBType)),
			sanitize(facts.NodeDBType), "NuDB or RocksDB")
	}
}

func peerPortCheck(config configState) model.Check {
	if !config.ok {
		return skipped("config.peer_port", configSkipReason(config))
	}

	var peers []xrpld.Listener
	for _, listener := range config.facts.Listeners() {
		if listener.HasProtocol("peer") {
			peers = append(peers, listener)
		}
	}

	switch len(peers) {
	case 0:
		return check("config.peer_port", model.StatusWarn,
			"no active listener serves the peer protocol", "0 peer listeners", "one peer listener")
	case 1:
	default:
		return check("config.peer_port", model.StatusFail,
			fmt.Sprintf("%d active listeners serve the peer protocol; xrpld supports one", len(peers)),
			fmt.Sprintf("%d peer listeners", len(peers)), "one peer listener")
	}

	port := strings.TrimSpace(peers[0].Port)
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return check("config.peer_port", model.StatusFail,
			fmt.Sprintf("peer listener %s has no valid port", quotePath(peers[0].Name)),
			sanitize(port), "a port between 1 and 65535")
	}

	return check("config.peer_port", model.StatusPass,
		fmt.Sprintf("peer listener on port %d (%s)", number, peerPortKind(port)),
		port, "a port between 1 and 65535")
}

func peerPortKind(port string) string {
	switch port {
	case ianaPeerPort:
		return "IANA-assigned XRP Ledger peer port"
	case legacyPeerPort:
		return "legacy XRP Ledger peer port"
	default:
		return "custom peer port"
	}
}

// configSkipReason explains why a configuration-dependent check was skipped.
func configSkipReason(config configState) string {
	if !config.supplied {
		return "no configuration file supplied"
	}
	return "configuration could not be read"
}
