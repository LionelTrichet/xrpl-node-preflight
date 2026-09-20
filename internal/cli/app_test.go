package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
)

// fakeProbe reports a healthy production host, so the CLI tests exercise the
// command line rather than the machine they run on.
type fakeProbe struct {
	cpu     int
	memory  uint64
	media   string
	ifaces  map[string]probe.NetworkInfo
	defName string
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{
		cpu:    16,
		memory: 64 * 1024 * 1024 * 1024,
		media:  probe.MediaSSD,
		ifaces: map[string]probe.NetworkInfo{
			"eth0": {Name: "eth0", OperState: "up", SpeedMbit: 1000},
		},
		defName: "eth0",
	}
}

func (f *fakeProbe) OS() string                   { return "linux" }
func (f *fakeProbe) Arch() string                 { return "amd64" }
func (f *fakeProbe) CPUCount() (int, error)       { return f.cpu, nil }
func (f *fakeProbe) MemoryTotal() (uint64, error) { return f.memory, nil }

func (f *fakeProbe) Storage(string) (probe.StorageInfo, error) {
	return probe.StorageInfo{
		TotalBytes:     500_000_000_000,
		AvailableBytes: 400_000_000_000,
		Media:          f.media,
	}, nil
}

func (f *fakeProbe) DefaultInterface() (string, error) { return f.defName, nil }

func (f *fakeProbe) NetworkInterface(name string) (probe.NetworkInfo, error) {
	info, ok := f.ifaces[name]
	if !ok {
		return probe.NetworkInfo{}, fmt.Errorf("no such interface %s", name)
	}
	return info, nil
}

func (f *fakeProbe) TimeSync(context.Context) (probe.TimeSyncInfo, error) {
	return probe.TimeSyncInfo{State: probe.TimeSynchronized, Source: "timedatectl"}, nil
}

func (f *fakeProbe) Nofile(context.Context) (probe.NofileInfo, error) {
	return probe.NofileInfo{ServiceVerified: true, ServiceLimit: 65536}, nil
}

func (f *fakeProbe) XRPLDVersion(context.Context) (probe.BinaryInfo, error) {
	return probe.BinaryInfo{Found: true, Path: "/usr/bin/xrpld", VersionOK: true, Version: "rippled version 2.6.1"}, nil
}

func invoke(p probe.Probe, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, p)
	return code, stdout.String(), stderr.String()
}

func readyConfig(t *testing.T) string {
	t.Helper()
	body := "[node_db]\ntype=NuDB\npath=/var/lib/xrpld/db\n\n[node_size]\nhuge\n\n" +
		"[server]\nport_peer\nport_rpc\n\n[port_peer]\nport=2459\nprotocol=peer\n\n" +
		"[port_rpc]\nip=127.0.0.1\nport=5005\nprotocol=http\n\n[validator_token]\ntoken-value\n"
	path := filepath.Join(t.TempDir(), "xrpld.cfg")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestUsageErrors(t *testing.T) {
	tests := [][]string{
		{},
		{"inspect"},
		{"check", "--role", "auditor"},
		{"check", "--level", "test"},
		{"check", "--format", "yaml"},
		{"check", "--unknown"},
		{"check", "extra"},
		{"check", "--config", ""},
		{"check", "--data-dir", ""},
		{"check", "--interface", ""},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, stdout, stderr := invoke(newFakeProbe(), args...)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "usage: xrpl-node-preflight check") {
				t.Errorf("stderr = %q, want the usage line", stderr)
			}
		})
	}
}

func TestReadyHostExitsZero(t *testing.T) {
	code, stdout, stderr := invoke(newFakeProbe(), "check", "--data-dir", "/srv/xrpld", "--config", readyConfig(t))
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "Result: READY\n") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestWarningsStillExitZero(t *testing.T) {
	p := newFakeProbe()
	p.media = probe.MediaUnknown
	code, stdout, _ := invoke(p, "check", "--data-dir", "/srv/xrpld", "--config", readyConfig(t))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for warnings", code)
	}
	if !strings.Contains(stdout, "Result: READY WITH WARNINGS") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestFailureExitsOne(t *testing.T) {
	p := newFakeProbe()
	p.cpu = 2
	code, stdout, _ := invoke(p, "check", "--data-dir", "/srv/xrpld")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "Result: NOT READY") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestJSONFormat(t *testing.T) {
	code, stdout, _ := invoke(newFakeProbe(), "check", "--format", "json",
		"--role", "validator", "--config", readyConfig(t), "--data-dir", "/srv/xrpld")
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stdout)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, output = %s", err, stdout)
	}
	if decoded["role"] != "validator" || decoded["level"] != "production" {
		t.Errorf("role/level = %v %v", decoded["role"], decoded["level"])
	}
	if checks, ok := decoded["checks"].([]any); !ok || len(checks) != 20 {
		t.Errorf("checks = %v", decoded["checks"])
	}
}

func TestDefaultsAreNodeAndProduction(t *testing.T) {
	_, stdout, _ := invoke(newFakeProbe(), "check", "--data-dir", "/srv/xrpld")
	if !strings.Contains(stdout, "Role:  node") || !strings.Contains(stdout, "Level: production") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestMinimumLevelIsAccepted(t *testing.T) {
	p := newFakeProbe()
	p.cpu = 4
	p.memory = 16 * 1024 * 1024 * 1024
	code, stdout, _ := invoke(p, "check", "--level", "minimum", "--data-dir", "/srv/xrpld")
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stdout)
	}
	if !strings.Contains(stdout, "Level: minimum") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestExplicitInterface(t *testing.T) {
	code, stdout, _ := invoke(newFakeProbe(), "check", "--interface", "eth9", "--data-dir", "/srv/xrpld")
	if code != 1 {
		t.Fatalf("exit code = %d, want the missing interface to fail", code)
	}
	if !strings.Contains(stdout, "eth9 does not exist") {
		t.Errorf("stdout = %s", stdout)
	}
}
