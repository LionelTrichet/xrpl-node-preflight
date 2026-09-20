// Package probe collects local Linux facts. It performs no network requests
// and changes nothing on the host; every function here only reads.
package probe

import "context"

// StorageInfo describes the filesystem holding the resolved storage target.
type StorageInfo struct {
	// Path is the existing path whose filesystem was measured, which may be
	// an ancestor of the requested target.
	Path           string
	TotalBytes     uint64
	AvailableBytes uint64
	// Media is "ssd", "rotational" or "unknown".
	Media string
}

// Media values reported by StorageInfo.
const (
	MediaSSD        = "ssd"
	MediaRotational = "rotational"
	MediaUnknown    = "unknown"
)

// NetworkInfo describes one network interface.
type NetworkInfo struct {
	Name string
	// OperState is the kernel operational state, or "" when unreadable.
	OperState string
	// SpeedMbit is the link speed in Mbit/s, or 0 when it is not known.
	SpeedMbit int
}

// TimeSyncInfo is the local synchronization state.
type TimeSyncInfo struct {
	// State is "synchronized", "unsynchronized" or "unknown".
	State string
	// Source names the probe that produced the state.
	Source string
}

// Time synchronization states.
const (
	TimeSynchronized   = "synchronized"
	TimeUnsynchronized = "unsynchronized"
	TimeUnknown        = "unknown"
)

// NofileInfo is the file-descriptor limit picture. A service limit is
// authoritative; the current process limit only indicates.
type NofileInfo struct {
	// ServiceVerified is true when xrpld.service is loaded and its
	// LimitNOFILE could be read.
	ServiceVerified bool
	ServiceLimit    uint64
	// ServiceInfinity is true when the service limit is unlimited.
	ServiceInfinity bool
	// ProcessLimit is the hard RLIMIT_NOFILE of this process.
	ProcessLimit uint64
	ProcessKnown bool
}

// BinaryInfo describes the locally installed xrpld binary.
type BinaryInfo struct {
	Found bool
	Path  string
	// Version is the sanitized first output line of `xrpld --version`.
	Version string
	// VersionOK is true when the version command succeeded.
	VersionOK bool
}

// Probe is the set of host facts the checks need. The real implementation
// reads Linux state; tests supply their own.
type Probe interface {
	OS() string
	Arch() string

	CPUCount() (int, error)
	MemoryTotal() (uint64, error)

	Storage(path string) (StorageInfo, error)

	DefaultInterface() (string, error)
	NetworkInterface(name string) (NetworkInfo, error)

	TimeSync(ctx context.Context) (TimeSyncInfo, error)
	Nofile(ctx context.Context) (NofileInfo, error)

	XRPLDVersion(ctx context.Context) (BinaryInfo, error)
}
