package probe

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Timeouts for the fixed local probes.
const (
	systemdTimeout = 2 * time.Second
	xrpldTimeout   = 3 * time.Second
)

// Linux reads readiness facts from the running Linux host. Every method only
// reads: no file is created, modified or removed, and nothing leaves the
// machine.
type Linux struct{}

// NewLinux returns a probe backed by the local host.
func NewLinux() Linux { return Linux{} }

// OS returns the operating system this binary was built for.
func (Linux) OS() string { return runtime.GOOS }

// Arch returns the architecture this binary was built for.
func (Linux) Arch() string { return runtime.GOARCH }

// CPUCount returns the number of logical CPUs available to this process. It
// is not a count of physical cores.
func (Linux) CPUCount() (int, error) { return runtime.NumCPU(), nil }

// MemoryTotal returns MemTotal from /proc/meminfo in bytes. Free and
// available memory are deliberately not consulted.
func (Linux) MemoryTotal() (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || key != "MemTotal" {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			break
		}
		kib, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemTotal: %w", err)
		}
		return kib * 1024, nil
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// Storage measures the filesystem holding path. When the path does not exist
// yet, the nearest existing ancestor is used; nothing is created.
func (Linux) Storage(path string) (StorageInfo, error) {
	existing, err := existingAncestor(path)
	if err != nil {
		return StorageInfo{}, err
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(existing, &stat); err != nil {
		return StorageInfo{}, fmt.Errorf("statfs %s: %w", existing, err)
	}

	info := StorageInfo{
		Path:           existing,
		TotalBytes:     stat.Blocks * uint64(stat.Bsize),
		AvailableBytes: stat.Bavail * uint64(stat.Bsize),
		Media:          MediaUnknown,
	}
	if media, err := mediaOf(existing); err == nil {
		info.Media = media
	}
	return info, nil
}

// existingAncestor walks upwards until it finds a path that exists.
func existingAncestor(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		if _, err := os.Stat(current); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing path for %s", path)
		}
		current = parent
	}
}

// mediaOf reports whether the kernel considers the backing device rotational.
func mediaOf(path string) (string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return MediaUnknown, fmt.Errorf("read mountinfo: %w", err)
	}

	entry, found := findMount(parseMountinfo(string(data)), path)
	if !found {
		return MediaUnknown, fmt.Errorf("no mount entry for %s", path)
	}

	device, err := filepath.EvalSymlinks(filepath.Join("/sys/dev/block", entry.Device))
	if err != nil {
		return MediaUnknown, fmt.Errorf("resolve device %s: %w", entry.Device, err)
	}

	// A partition has no queue of its own; its parent block device does.
	for current := device; strings.HasPrefix(current, "/sys"); current = filepath.Dir(current) {
		value, err := os.ReadFile(filepath.Join(current, "queue", "rotational"))
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(value)) {
		case "0":
			return MediaSSD, nil
		case "1":
			return MediaRotational, nil
		default:
			return MediaUnknown, nil
		}
	}
	return MediaUnknown, fmt.Errorf("no rotational attribute for %s", device)
}

// DefaultInterface returns the interface of the IPv4 default route.
func (Linux) DefaultInterface() (string, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", fmt.Errorf("read /proc/net/route: %w", err)
	}
	return parseDefaultRoute(string(data))
}

// NetworkInterface reports the link state and speed of one interface. The
// name is validated through the kernel interface list before it is used to
// build a sysfs path.
func (Linux) NetworkInterface(name string) (NetworkInfo, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return NetworkInfo{}, fmt.Errorf("interface %s: %w", name, err)
	}

	info := NetworkInfo{Name: iface.Name}
	base := filepath.Join("/sys/class/net", iface.Name)

	if state, err := os.ReadFile(filepath.Join(base, "operstate")); err == nil {
		info.OperState = strings.TrimSpace(string(state))
	}
	if speed, err := os.ReadFile(filepath.Join(base, "speed")); err == nil {
		if value, err := strconv.Atoi(strings.TrimSpace(string(speed))); err == nil && value > 0 {
			info.SpeedMbit = value
		}
	}
	return info, nil
}

// TimeSync reports the local synchronization state. It never queries a time
// server: only the local daemon state is inspected.
func (Linux) TimeSync(ctx context.Context) (TimeSyncInfo, error) {
	output, err := runCommand(ctx, systemdTimeout, "timedatectl", "show", "--property=NTPSynchronized", "--value")
	if err == nil {
		switch strings.ToLower(strings.TrimSpace(output)) {
		case "yes", "true":
			return TimeSyncInfo{State: TimeSynchronized, Source: "timedatectl"}, nil
		case "no", "false":
			return TimeSyncInfo{State: TimeUnsynchronized, Source: "timedatectl"}, nil
		}
	}

	output, err = runCommand(ctx, systemdTimeout, "chronyc", "tracking")
	if err == nil {
		lowered := strings.ToLower(output)
		switch {
		case strings.Contains(lowered, "not synchronised"), strings.Contains(lowered, "not synchronized"):
			return TimeSyncInfo{State: TimeUnsynchronized, Source: "chronyc"}, nil
		case strings.Contains(lowered, "leap status") && strings.Contains(lowered, "normal"):
			return TimeSyncInfo{State: TimeSynchronized, Source: "chronyc"}, nil
		}
	}

	return TimeSyncInfo{State: TimeUnknown, Source: ""}, nil
}

// Nofile reports the file-descriptor limits. The xrpld service limit is
// authoritative when it can be read; the current process limit is only an
// indication.
func (Linux) Nofile(ctx context.Context) (NofileInfo, error) {
	info := NofileInfo{}

	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err == nil {
		info.ProcessLimit = limit.Max
		info.ProcessKnown = true
	}

	output, err := runCommand(ctx, systemdTimeout, "systemctl", "show", "xrpld.service",
		"--property=LoadState", "--property=LimitNOFILE", "--no-pager")
	if err != nil {
		return info, nil
	}

	loaded := false
	value := ""
	for _, line := range strings.Split(output, "\n") {
		key, raw, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		switch key {
		case "LoadState":
			loaded = strings.EqualFold(strings.TrimSpace(raw), "loaded")
		case "LimitNOFILE":
			value = strings.TrimSpace(raw)
		}
	}
	if !loaded || value == "" {
		return info, nil
	}

	switch strings.ToLower(value) {
	case "infinity", "infinite":
		info.ServiceVerified = true
		info.ServiceInfinity = true
		return info, nil
	}
	if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
		info.ServiceVerified = true
		info.ServiceLimit = parsed
	}
	return info, nil
}

// XRPLDVersion looks for a local xrpld binary and asks it for its version. No
// release or package index is ever contacted.
func (Linux) XRPLDVersion(ctx context.Context) (BinaryInfo, error) {
	path, err := exec.LookPath("xrpld")
	if err != nil {
		return BinaryInfo{Found: false}, nil
	}

	info := BinaryInfo{Found: true, Path: path}
	output, err := runCommand(ctx, xrpldTimeout, "xrpld", "--version")
	if err != nil {
		return info, nil
	}
	info.VersionOK = true
	info.Version = firstDisplayLine(output)
	return info, nil
}
