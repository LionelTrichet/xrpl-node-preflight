package probe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseMountinfo(t *testing.T) {
	data := `24 30 0:22 / /sys rw,nosuid shared:7 - sysfs sysfs rw
30 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw
41 30 8:16 / /var/lib rw,relatime shared:25 - ext4 /dev/sdb rw
42 30 8:32 / /var/library rw,relatime shared:26 - ext4 /dev/sdc rw
`
	entries := parseMountinfo(data)
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}

	tests := []struct {
		path   string
		device string
	}{
		{"/", "8:1"},
		{"/home/user", "8:1"},
		{"/var/lib", "8:16"},
		{"/var/lib/xrpld/db", "8:16"},
		// A prefix that is not a path component must not match.
		{"/var/library/books", "8:32"},
		{"/var/libfoo", "8:1"},
	}
	for _, test := range tests {
		entry, found := findMount(entries, test.path)
		if !found {
			t.Fatalf("%s: no mount found", test.path)
		}
		if entry.Device != test.device {
			t.Errorf("%s: device = %s, want %s", test.path, entry.Device, test.device)
		}
	}
}

func TestMountinfoEscapes(t *testing.T) {
	data := `30 1 8:1 / / rw - ext4 /dev/sda1 rw
41 30 8:16 / /mnt/with\040space rw - ext4 /dev/sdb rw
42 30 8:32 / /mnt/with\011tab rw - ext4 /dev/sdc rw
43 30 8:48 / /mnt/with\134backslash rw - ext4 /dev/sdd rw
44 30 8:64 / /mnt/with\012newline rw - ext4 /dev/sde rw
`
	entries := parseMountinfo(data)
	points := make([]string, 0, len(entries))
	for _, entry := range entries {
		points = append(points, entry.MountPoint)
	}

	want := []string{"/", "/mnt/with space", "/mnt/with\ttab", `/mnt/with\backslash`, "/mnt/with\nnewline"}
	for index, expected := range want {
		if points[index] != expected {
			t.Errorf("mount %d = %q, want %q", index, points[index], expected)
		}
	}

	entry, found := findMount(entries, "/mnt/with space/db")
	if !found || entry.Device != "8:16" {
		t.Errorf("escaped mount lookup = %+v, found = %v", entry, found)
	}
}

func TestParseMountinfoIgnoresShortLines(t *testing.T) {
	entries := parseMountinfo("garbage\n30 1 8:1 / / rw - ext4 /dev/sda1 rw\n1 2 no-device / / rw\n")
	if len(entries) != 1 {
		t.Fatalf("entries = %+v, want only the well-formed line", entries)
	}
}

func TestParseDefaultRoute(t *testing.T) {
	data := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth1	00000000	0102030A	0003	0	0	200	00000000	0	0	0
eth0	00000000	0102030A	0003	0	0	100	00000000	0	0	0
eth2	0002A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`
	name, err := parseDefaultRoute(data)
	if err != nil {
		t.Fatalf("parseDefaultRoute() error = %v", err)
	}
	if name != "eth0" {
		t.Errorf("interface = %s, want eth0 (the lowest metric)", name)
	}
}

func TestDefaultRouteRequiresUpFlag(t *testing.T) {
	data := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0102030A	0002	0	0	0	00000000	0	0	0
`
	if _, err := parseDefaultRoute(data); err == nil {
		t.Fatal("parseDefaultRoute() error = nil, want no usable route")
	}
}

func TestNoDefaultRoute(t *testing.T) {
	data := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	0002A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`
	if _, err := parseDefaultRoute(data); err == nil {
		t.Fatal("parseDefaultRoute() error = nil, want no default route")
	}
}

func TestCappedWriterBoundsMemoryButKeepsDraining(t *testing.T) {
	writer := newCappedWriter(10)

	n, err := writer.Write([]byte("0123456789abcdef"))
	if err != nil || n != 16 {
		t.Fatalf("Write() = %d, %v, want the whole slice accepted", n, err)
	}
	if got := writer.String(); got != "0123456789" {
		t.Errorf("stored = %q, want the first ten bytes", got)
	}
	if !writer.Truncated() {
		t.Error("Truncated() = false, want true")
	}

	if n, err := writer.Write([]byte("more")); n != 4 || err != nil {
		t.Errorf("Write() after the cap = %d, %v", n, err)
	}
	if len(writer.String()) != 10 {
		t.Errorf("stored = %d bytes, want it to stay at the cap", len(writer.String()))
	}
}

func TestCappedWriterIsConcurrencySafe(t *testing.T) {
	writer := newCappedWriter(1024)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				writer.Write([]byte("xxxxxxxx"))
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if len(writer.String()) > 1024 {
		t.Errorf("stored = %d bytes, want at most the cap", len(writer.String()))
	}
}

func TestRunCommandBoundsOutput(t *testing.T) {
	// Produce far more than the cap through a fixed local command.
	output, err := runCommand(context.Background(), 5*time.Second, "head", "-c", "200000", "/dev/zero")
	if err != nil {
		t.Skipf("head is unavailable: %v", err)
	}
	if len(output) > maxCommandOutput {
		t.Errorf("output = %d bytes, want at most %d", len(output), maxCommandOutput)
	}
}

func TestRunCommandTimeout(t *testing.T) {
	start := time.Now()
	_, err := runCommand(context.Background(), 100*time.Millisecond, "sleep", "5")
	if err == nil {
		t.Fatal("runCommand() error = nil, want a timeout")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("elapsed = %v, want the timeout to stop the child", elapsed)
	}
}

func TestSanitizeDisplay(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"escape sequence", "xrpld \x1b[31mversion\x1b[0m 2.6.1", "xrpld [31mversion [0m 2.6.1"},
		{"carriage return", "first\rsecond", "first second"},
		{"tabs collapse", "a\t\tb", "a b"},
		{"bell", "ver\asion", "ver sion"},
	}
	for _, test := range tests {
		if got := sanitizeDisplay(test.input); got != test.want {
			t.Errorf("%s: sanitizeDisplay() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestFirstDisplayLine(t *testing.T) {
	if got := firstDisplayLine("\n\n  rippled version 2.6.1\nbuild info\n"); got != "rippled version 2.6.1" {
		t.Errorf("firstDisplayLine() = %q", got)
	}
	if got := firstDisplayLine("\x00\x01\n"); got != "" {
		t.Errorf("firstDisplayLine() = %q, want empty", got)
	}
}

func TestExistingAncestor(t *testing.T) {
	directory := t.TempDir()
	deep := filepath.Join(directory, "a", "b", "c")

	found, err := existingAncestor(deep)
	if err != nil {
		t.Fatalf("existingAncestor() error = %v", err)
	}
	if found != directory {
		t.Errorf("ancestor = %s, want %s", found, directory)
	}
	// Nothing may be created while looking for the ancestor.
	if _, err := os.Stat(filepath.Join(directory, "a")); err == nil {
		t.Error("existingAncestor created a directory")
	}
}

func TestLinuxProbeReadsTheHost(t *testing.T) {
	linux := NewLinux()
	if linux.OS() == "" || linux.Arch() == "" {
		t.Fatal("OS and Arch must be reported")
	}

	count, err := linux.CPUCount()
	if err != nil || count < 1 {
		t.Fatalf("CPUCount() = %d, %v", count, err)
	}

	total, err := linux.MemoryTotal()
	if err != nil {
		t.Fatalf("MemoryTotal() error = %v", err)
	}
	if total == 0 {
		t.Error("MemoryTotal() = 0")
	}

	storage, err := linux.Storage("/")
	if err != nil {
		t.Fatalf("Storage() error = %v", err)
	}
	if storage.TotalBytes == 0 {
		t.Error("TotalBytes = 0")
	}
	switch storage.Media {
	case MediaSSD, MediaRotational, MediaUnknown:
	default:
		t.Errorf("Media = %q", storage.Media)
	}

	info, err := linux.Nofile(context.Background())
	if err != nil {
		t.Fatalf("Nofile() error = %v", err)
	}
	if !info.ProcessKnown {
		t.Error("the process limit should always be readable")
	}

	sync, err := linux.TimeSync(context.Background())
	if err != nil {
		t.Fatalf("TimeSync() error = %v", err)
	}
	switch sync.State {
	case TimeSynchronized, TimeUnsynchronized, TimeUnknown:
	default:
		t.Errorf("State = %q", sync.State)
	}

	binary, err := linux.XRPLDVersion(context.Background())
	if err != nil {
		t.Fatalf("XRPLDVersion() error = %v", err)
	}
	if binary.Found && strings.TrimSpace(binary.Path) == "" {
		t.Error("a found binary must report its path")
	}
}
