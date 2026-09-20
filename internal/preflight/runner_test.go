package preflight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
)

// fakeProbe supplies host facts, so the verdicts never depend on the machine
// the tests run on.
type fakeProbe struct {
	os           string
	arch         string
	cpu          int
	cpuErr       error
	memory       uint64
	memoryErr    error
	storage      probe.StorageInfo
	storageErr   error
	storagePaths []string
	defaultIface string
	defaultErr   error
	interfaces   map[string]probe.NetworkInfo
	timeSync     probe.TimeSyncInfo
	timeErr      error
	nofile       probe.NofileInfo
	nofileErr    error
	binary       probe.BinaryInfo
	binaryErr    error
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{
		os:     "linux",
		arch:   "amd64",
		cpu:    16,
		memory: 64 * gibibyte,
		storage: probe.StorageInfo{
			Path:           "/var/lib/xrpld/db",
			TotalBytes:     500_000_000_000,
			AvailableBytes: 400_000_000_000,
			Media:          probe.MediaSSD,
		},
		defaultIface: "eth0",
		interfaces: map[string]probe.NetworkInfo{
			"eth0": {Name: "eth0", OperState: "up", SpeedMbit: 1000},
		},
		timeSync: probe.TimeSyncInfo{State: probe.TimeSynchronized, Source: "timedatectl"},
		nofile:   probe.NofileInfo{ServiceVerified: true, ServiceLimit: 65536},
		binary:   probe.BinaryInfo{Found: true, Path: "/usr/bin/xrpld", VersionOK: true, Version: "rippled version 2.6.1"},
	}
}

func (f *fakeProbe) OS() string             { return f.os }
func (f *fakeProbe) Arch() string           { return f.arch }
func (f *fakeProbe) CPUCount() (int, error) { return f.cpu, f.cpuErr }
func (f *fakeProbe) MemoryTotal() (uint64, error) {
	return f.memory, f.memoryErr
}

func (f *fakeProbe) Storage(path string) (probe.StorageInfo, error) {
	f.storagePaths = append(f.storagePaths, path)
	return f.storage, f.storageErr
}

func (f *fakeProbe) DefaultInterface() (string, error) { return f.defaultIface, f.defaultErr }

func (f *fakeProbe) NetworkInterface(name string) (probe.NetworkInfo, error) {
	info, ok := f.interfaces[name]
	if !ok {
		return probe.NetworkInfo{}, fmt.Errorf("no such interface %s", name)
	}
	return info, nil
}

func (f *fakeProbe) TimeSync(context.Context) (probe.TimeSyncInfo, error) {
	return f.timeSync, f.timeErr
}

func (f *fakeProbe) Nofile(context.Context) (probe.NofileInfo, error) {
	return f.nofile, f.nofileErr
}

func (f *fakeProbe) XRPLDVersion(context.Context) (probe.BinaryInfo, error) {
	return f.binary, f.binaryErr
}

func run(p probe.Probe, opts Options) model.Report {
	if opts.Role == "" {
		opts.Role = RoleNode
	}
	if opts.Level == "" {
		opts.Level = LevelProduction
	}
	return Run(context.Background(), p, opts)
}

func find(t *testing.T, report model.Report, id string) model.Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %s is missing from the report", id)
	return model.Check{}
}

func assertStatus(t *testing.T, report model.Report, id string, want model.Status) model.Check {
	t.Helper()
	check := find(t, report, id)
	if check.Status != want {
		t.Fatalf("%s = %s (%s), want %s", id, check.Status, check.Summary, want)
	}
	return check
}

func testdata(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "xrpld", "testdata", name)
}

func TestReportOrderAndShape(t *testing.T) {
	report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: testdata(t, "validator-token.cfg")})

	want := []string{
		"host.os", "host.arch", "host.cpu", "host.memory",
		"storage.target", "storage.capacity", "storage.media",
		"network.interface", "time.sync", "limits.nofile",
		"xrpld.binary",
		"config.file", "config.node_size", "config.node_db", "config.peer_port",
		"validator.config", "validator.credentials", "validator.legacy_seed",
		"validator.config_mode", "validator.api_bind",
	}

	got := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		got = append(got, check.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("check order =\n%v\nwant\n%v", got, want)
	}
	if report.SchemaVersion != model.SchemaVersion {
		t.Errorf("schemaVersion = %d", report.SchemaVersion)
	}
}

func TestNodeRoleHasNoValidatorChecks(t *testing.T) {
	report := run(newFakeProbe(), Options{})
	for _, check := range report.Checks {
		if strings.HasPrefix(check.ID, "validator.") {
			t.Fatalf("node role produced %s", check.ID)
		}
	}
}

func TestHostChecks(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		setup  func(*fakeProbe)
		id     string
		status model.Status
	}{
		{"linux", LevelProduction, func(*fakeProbe) {}, "host.os", model.StatusPass},
		{"not linux", LevelProduction, func(p *fakeProbe) { p.os = "darwin" }, "host.os", model.StatusFail},

		{"production amd64", LevelProduction, func(*fakeProbe) {}, "host.arch", model.StatusPass},
		{"production arm64", LevelProduction, func(p *fakeProbe) { p.arch = "arm64" }, "host.arch", model.StatusFail},
		{"minimum amd64", LevelMinimum, func(*fakeProbe) {}, "host.arch", model.StatusPass},
		{"minimum arm64", LevelMinimum, func(p *fakeProbe) { p.arch = "arm64" }, "host.arch", model.StatusWarn},
		{"minimum 386", LevelMinimum, func(p *fakeProbe) { p.arch = "386" }, "host.arch", model.StatusFail},

		{"production 8 cpus", LevelProduction, func(p *fakeProbe) { p.cpu = 8 }, "host.cpu", model.StatusPass},
		{"production 7 cpus", LevelProduction, func(p *fakeProbe) { p.cpu = 7 }, "host.cpu", model.StatusFail},
		{"minimum 4 cpus", LevelMinimum, func(p *fakeProbe) { p.cpu = 4 }, "host.cpu", model.StatusPass},
		{"minimum 3 cpus", LevelMinimum, func(p *fakeProbe) { p.cpu = 3 }, "host.cpu", model.StatusFail},
		{"cpu error", LevelProduction, func(p *fakeProbe) { p.cpuErr = errors.New("no") }, "host.cpu", model.StatusWarn},

		{"production 64 GiB", LevelProduction, func(p *fakeProbe) { p.memory = 64 * gibibyte }, "host.memory", model.StatusPass},
		{"production just below", LevelProduction, func(p *fakeProbe) { p.memory = 64*gibibyte - 1 }, "host.memory", model.StatusFail},
		{"minimum 16 GiB", LevelMinimum, func(p *fakeProbe) { p.memory = 16 * gibibyte }, "host.memory", model.StatusPass},
		{"minimum just below", LevelMinimum, func(p *fakeProbe) { p.memory = 16*gibibyte - 1 }, "host.memory", model.StatusFail},
		{"memory error", LevelProduction, func(p *fakeProbe) { p.memoryErr = errors.New("no") }, "host.memory", model.StatusWarn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newFakeProbe()
			test.setup(p)
			report := run(p, Options{Level: test.level})
			assertStatus(t, report, test.id, test.status)
		})
	}
}

func TestCPUWordingIsLogical(t *testing.T) {
	check := find(t, run(newFakeProbe(), Options{}), "host.cpu")
	if !strings.Contains(check.Summary, "logical CPUs") {
		t.Errorf("summary = %q, want logical CPUs", check.Summary)
	}
	if strings.Contains(strings.ToLower(check.Summary), "physical") {
		t.Errorf("summary = %q, must not claim physical cores", check.Summary)
	}
}

func TestStorageTargetPrecedence(t *testing.T) {
	config := testdata(t, "node.cfg")

	t.Run("explicit data dir wins", func(t *testing.T) {
		p := newFakeProbe()
		report := run(p, Options{ConfigPath: config, DataDir: "/srv/xrpld"})
		assertStatus(t, report, "storage.target", model.StatusPass)
		if p.storagePaths[0] != "/srv/xrpld" {
			t.Errorf("measured %q", p.storagePaths[0])
		}
	})

	t.Run("relative data dir is relative to the working directory", func(t *testing.T) {
		p := newFakeProbe()
		run(p, Options{DataDir: "relative/db"})
		wd, _ := os.Getwd()
		if want := filepath.Join(wd, "relative/db"); p.storagePaths[0] != want {
			t.Errorf("measured %q, want %q", p.storagePaths[0], want)
		}
	})

	t.Run("absolute NodeDB path", func(t *testing.T) {
		p := newFakeProbe()
		run(p, Options{ConfigPath: config})
		if p.storagePaths[0] != "/var/lib/xrpld/db/nudb" {
			t.Errorf("measured %q", p.storagePaths[0])
		}
	})

	t.Run("relative NodeDB path is relative to the config", func(t *testing.T) {
		p := newFakeProbe()
		relative := testdata(t, "server-defaults.cfg")
		run(p, Options{ConfigPath: relative})
		want := filepath.Join(filepath.Dir(relative), "db/nudb")
		if p.storagePaths[0] != want {
			t.Errorf("measured %q, want %q", p.storagePaths[0], want)
		}
	})

	t.Run("no config falls back to root", func(t *testing.T) {
		p := newFakeProbe()
		report := run(p, Options{})
		assertStatus(t, report, "storage.target", model.StatusWarn)
		if p.storagePaths[0] != "/" {
			t.Errorf("measured %q", p.storagePaths[0])
		}
	})

	t.Run("broken config falls back to root but keeps host checks", func(t *testing.T) {
		p := newFakeProbe()
		report := run(p, Options{ConfigPath: testdata(t, "malformed.cfg")})
		assertStatus(t, report, "storage.target", model.StatusWarn)
		assertStatus(t, report, "storage.capacity", model.StatusPass)
		assertStatus(t, report, "host.cpu", model.StatusPass)
		if p.storagePaths[0] != "/" {
			t.Errorf("measured %q", p.storagePaths[0])
		}
	})
}

func TestStorageVerdicts(t *testing.T) {
	tests := []struct {
		name    string
		storage probe.StorageInfo
		err     error
		id      string
		status  model.Status
	}{
		{"exactly the threshold", probe.StorageInfo{TotalBytes: requiredCapacity, Media: probe.MediaSSD}, nil, "storage.capacity", model.StatusPass},
		{"one byte below", probe.StorageInfo{TotalBytes: requiredCapacity - 1, Media: probe.MediaSSD}, nil, "storage.capacity", model.StatusFail},
		{"capacity unknown", probe.StorageInfo{}, errors.New("no"), "storage.capacity", model.StatusWarn},
		{"non-rotational", probe.StorageInfo{TotalBytes: requiredCapacity, Media: probe.MediaSSD}, nil, "storage.media", model.StatusPass},
		{"rotational", probe.StorageInfo{TotalBytes: requiredCapacity, Media: probe.MediaRotational}, nil, "storage.media", model.StatusFail},
		{"media unknown", probe.StorageInfo{TotalBytes: requiredCapacity, Media: probe.MediaUnknown}, nil, "storage.media", model.StatusWarn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newFakeProbe()
			p.storage = test.storage
			p.storageErr = test.err
			assertStatus(t, run(p, Options{}), test.id, test.status)
		})
	}
}

func TestStorageMediaWordingIsModest(t *testing.T) {
	check := find(t, run(newFakeProbe(), Options{}), "storage.media")
	for _, forbidden := range []string{"IOPS", "certified", "EBS", "latency"} {
		if strings.Contains(check.Summary, forbidden) {
			t.Errorf("summary = %q, must not claim %q", check.Summary, forbidden)
		}
	}
}

func TestNetworkChecks(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		opts   Options
		setup  func(*fakeProbe)
		status model.Status
	}{
		{"explicit interface", LevelProduction, Options{Interface: "eth0"}, func(*fakeProbe) {}, model.StatusPass},
		{"explicit missing interface", LevelProduction, Options{Interface: "eth9"}, func(*fakeProbe) {}, model.StatusFail},
		{"no default route", LevelProduction, Options{}, func(p *fakeProbe) { p.defaultErr = errors.New("none") }, model.StatusWarn},
		{"interface down", LevelProduction, Options{}, func(p *fakeProbe) {
			p.interfaces["eth0"] = probe.NetworkInfo{Name: "eth0", OperState: "down", SpeedMbit: 1000}
		}, model.StatusFail},
		{"interface state unknown", LevelProduction, Options{}, func(p *fakeProbe) {
			p.interfaces["eth0"] = probe.NetworkInfo{Name: "eth0", OperState: "unknown", SpeedMbit: 1000}
		}, model.StatusWarn},
		{"production 1000 Mbit", LevelProduction, Options{}, func(*fakeProbe) {}, model.StatusPass},
		{"production 999 Mbit", LevelProduction, Options{}, func(p *fakeProbe) {
			p.interfaces["eth0"] = probe.NetworkInfo{Name: "eth0", OperState: "up", SpeedMbit: 999}
		}, model.StatusFail},
		{"minimum 100 Mbit", LevelMinimum, Options{}, func(p *fakeProbe) {
			p.interfaces["eth0"] = probe.NetworkInfo{Name: "eth0", OperState: "up", SpeedMbit: 100}
		}, model.StatusWarn},
		{"speed unknown", LevelProduction, Options{}, func(p *fakeProbe) {
			p.interfaces["eth0"] = probe.NetworkInfo{Name: "eth0", OperState: "up"}
		}, model.StatusWarn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newFakeProbe()
			test.setup(p)
			opts := test.opts
			opts.Level = test.level
			assertStatus(t, run(p, opts), "network.interface", test.status)
		})
	}
}

func TestNetworkReportsNoAddresses(t *testing.T) {
	check := find(t, run(newFakeProbe(), Options{}), "network.interface")
	rendered := check.Summary + check.Observed + check.Expected
	for _, forbidden := range []string{"192.", "10.", "::", "gateway", "MAC"} {
		if strings.Contains(rendered, forbidden) {
			t.Errorf("network check leaked %q in %q", forbidden, rendered)
		}
	}
}

func TestTimeChecks(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		info   probe.TimeSyncInfo
		err    error
		status model.Status
	}{
		{"synchronized", LevelProduction, probe.TimeSyncInfo{State: probe.TimeSynchronized, Source: "timedatectl"}, nil, model.StatusPass},
		{"production unsynchronized", LevelProduction, probe.TimeSyncInfo{State: probe.TimeUnsynchronized, Source: "chronyc"}, nil, model.StatusFail},
		{"minimum unsynchronized", LevelMinimum, probe.TimeSyncInfo{State: probe.TimeUnsynchronized}, nil, model.StatusWarn},
		{"unknown", LevelProduction, probe.TimeSyncInfo{State: probe.TimeUnknown}, nil, model.StatusWarn},
		{"probe error", LevelProduction, probe.TimeSyncInfo{}, errors.New("no"), model.StatusWarn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newFakeProbe()
			p.timeSync = test.info
			p.timeErr = test.err
			assertStatus(t, run(p, Options{Level: test.level}), "time.sync", test.status)
		})
	}
}

func TestNofileChecks(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		info   probe.NofileInfo
		err    error
		status model.Status
		want   string
	}{
		{"service at the threshold", LevelProduction, probe.NofileInfo{ServiceVerified: true, ServiceLimit: 65536}, nil, model.StatusPass, "65536"},
		{"service below", LevelProduction, probe.NofileInfo{ServiceVerified: true, ServiceLimit: 1024}, nil, model.StatusFail, "1024"},
		{"minimum service below", LevelMinimum, probe.NofileInfo{ServiceVerified: true, ServiceLimit: 1024}, nil, model.StatusWarn, "1024"},
		{"service infinity", LevelProduction, probe.NofileInfo{ServiceVerified: true, ServiceInfinity: true}, nil, model.StatusPass, "unlimited"},
		{"process limit high", LevelProduction, probe.NofileInfo{ProcessKnown: true, ProcessLimit: 1048576}, nil, model.StatusWarn, "could not be verified"},
		{"process limit low", LevelProduction, probe.NofileInfo{ProcessKnown: true, ProcessLimit: 1024}, nil, model.StatusWarn, "could not be verified"},
		{"nothing known", LevelProduction, probe.NofileInfo{}, nil, model.StatusWarn, "could not be verified"},
		{"probe error", LevelProduction, probe.NofileInfo{}, errors.New("timeout"), model.StatusWarn, "could not be verified"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newFakeProbe()
			p.nofile = test.info
			p.nofileErr = test.err
			check := assertStatus(t, run(p, Options{Level: test.level}), "limits.nofile", test.status)
			if !strings.Contains(check.Summary, test.want) {
				t.Errorf("summary = %q, want it to mention %q", check.Summary, test.want)
			}
		})
	}
}

func TestBinaryChecks(t *testing.T) {
	tests := []struct {
		name   string
		info   probe.BinaryInfo
		err    error
		status model.Status
		want   string
	}{
		{"version reported", probe.BinaryInfo{Found: true, VersionOK: true, Version: "rippled version 2.6.1"}, nil, model.StatusPass, "2.6.1"},
		{"binary absent", probe.BinaryInfo{}, nil, model.StatusWarn, "not found in PATH"},
		{"version failed", probe.BinaryInfo{Found: true, Path: "/usr/bin/xrpld"}, nil, model.StatusWarn, "did not report a version"},
		{"probe error", probe.BinaryInfo{}, errors.New("timeout"), model.StatusWarn, "could not be determined"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newFakeProbe()
			p.binary = test.info
			p.binaryErr = test.err
			check := assertStatus(t, run(p, Options{}), "xrpld.binary", test.status)
			if !strings.Contains(check.Summary, test.want) {
				t.Errorf("summary = %q, want %q", check.Summary, test.want)
			}
		})
	}
}

func TestBinaryVersionIsSanitized(t *testing.T) {
	p := newFakeProbe()
	p.binary = probe.BinaryInfo{Found: true, VersionOK: true, Version: "rippled\x1b[31m version\n2.6.1"}
	check := find(t, run(p, Options{}), "xrpld.binary")
	if strings.ContainsAny(check.Summary, "\n\r\x1b") {
		t.Fatalf("summary = %q, want control characters removed", check.Summary)
	}
}

func TestConfigChecksWithoutConfig(t *testing.T) {
	report := run(newFakeProbe(), Options{})
	for _, id := range []string{"config.file", "config.node_size", "config.node_db", "config.peer_port"} {
		assertStatus(t, report, id, model.StatusSkip)
	}
}

func TestNodeSizeChecks(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		value  string
		memory uint64
		status model.Status
	}{
		{"absent", LevelProduction, "", 64 * gibibyte, model.StatusPass},
		{"production huge", LevelProduction, "huge", 64 * gibibyte, model.StatusPass},
		{"production mixed case", LevelProduction, "HuGe", 64 * gibibyte, model.StatusPass},
		{"production large", LevelProduction, "large", 64 * gibibyte, model.StatusWarn},
		{"production medium", LevelProduction, "medium", 64 * gibibyte, model.StatusWarn},
		{"production tiny", LevelProduction, "tiny", 64 * gibibyte, model.StatusWarn},
		{"invalid", LevelProduction, "enormous", 64 * gibibyte, model.StatusFail},
		{"minimum small", LevelMinimum, "small", 16 * gibibyte, model.StatusPass},
		{"minimum large", LevelMinimum, "large", 16 * gibibyte, model.StatusWarn},
		{"minimum huge with 32 GiB", LevelMinimum, "huge", 32 * gibibyte, model.StatusPass},
		{"minimum huge below 32 GiB", LevelMinimum, "huge", 31 * gibibyte, model.StatusWarn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, fmt.Sprintf("[node_db]\ntype=NuDB\npath=/db\n%s", nodeSizeSection(test.value)))
			p := newFakeProbe()
			p.memory = test.memory
			assertStatus(t, run(p, Options{Level: test.level, ConfigPath: path}), "config.node_size", test.status)
		})
	}
}

func TestNodeDBChecks(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status model.Status
	}{
		{"NuDB", "[node_db]\ntype=NuDB\npath=/db\n", model.StatusPass},
		{"lowercase nudb", "[node_db]\nTYPE=nudb\nPATH=/db\n", model.StatusPass},
		{"RocksDB", "[node_db]\ntype=RocksDB\npath=/db\n", model.StatusWarn},
		{"unknown type", "[node_db]\ntype=LevelDB\npath=/db\n", model.StatusFail},
		{"missing type", "[node_db]\npath=/db\n", model.StatusFail},
		{"missing path", "[node_db]\ntype=NuDB\n", model.StatusFail},
		{"missing section", "[server]\nport_peer\n", model.StatusFail},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := run(newFakeProbe(), Options{ConfigPath: writeConfig(t, test.body)})
			assertStatus(t, report, "config.node_db", test.status)
		})
	}
}

func TestRocksDBWordingIsNotUnsupported(t *testing.T) {
	report := run(newFakeProbe(), Options{ConfigPath: writeConfig(t, "[node_db]\ntype=RocksDB\npath=/db\n")})
	check := find(t, report, "config.node_db")
	if strings.Contains(strings.ToLower(check.Summary), "unsupported") {
		t.Errorf("summary = %q, RocksDB is legacy, not unsupported", check.Summary)
	}
}

func TestPeerPortChecks(t *testing.T) {
	base := "[node_db]\ntype=NuDB\npath=/db\n"
	tests := []struct {
		name   string
		body   string
		status model.Status
		want   string
	}{
		{"no peer listener", base + "[server]\nport_rpc\n\n[port_rpc]\nport=5005\nprotocol=http\n", model.StatusWarn, "no active listener"},
		{"IANA port", base + "[server]\nport_peer\n\n[port_peer]\nport=2459\nprotocol=peer\n", model.StatusPass, "IANA-assigned"},
		{"legacy port", base + "[server]\nport_peer\n\n[port_peer]\nport=51235\nprotocol=peer\n", model.StatusPass, "legacy"},
		{"custom port", base + "[server]\nport_peer\n\n[port_peer]\nport=12345\nprotocol=peer\n", model.StatusPass, "custom"},
		{"port inherited from server", base + "[server]\nport_peer\nport=2459\n\n[port_peer]\nprotocol=peer\n", model.StatusPass, "2459"},
		{"protocol inherited from server", base + "[server]\nport_peer\nprotocol=peer\n\n[port_peer]\nport=2459\n", model.StatusPass, "2459"},
		{"multi protocol", base + "[server]\nport_peer\n\n[port_peer]\nport=2459\nprotocol=peer,https\n", model.StatusPass, "2459"},
		{"missing port", base + "[server]\nport_peer\n\n[port_peer]\nprotocol=peer\n", model.StatusFail, "no valid port"},
		{"port zero", base + "[server]\nport_peer\n\n[port_peer]\nport=0\nprotocol=peer\n", model.StatusFail, "no valid port"},
		{"port too large", base + "[server]\nport_peer\n\n[port_peer]\nport=65536\nprotocol=peer\n", model.StatusFail, "no valid port"},
		{
			"two peer listeners",
			base + "[server]\nport_peer\nport_peer2\n\n[port_peer]\nport=2459\nprotocol=peer\n\n[port_peer2]\nport=51235\nprotocol=peer\n",
			model.StatusFail,
			"xrpld supports one",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := run(newFakeProbe(), Options{ConfigPath: writeConfig(t, test.body)})
			check := assertStatus(t, report, "config.peer_port", test.status)
			if !strings.Contains(check.Summary, test.want) {
				t.Errorf("summary = %q, want %q", check.Summary, test.want)
			}
		})
	}
}

func TestConfigFailureCascade(t *testing.T) {
	report := run(newFakeProbe(), Options{
		Role:       RoleValidator,
		ConfigPath: testdata(t, "malformed.cfg"),
	})

	assertStatus(t, report, "config.file", model.StatusFail)
	for _, id := range []string{"config.node_size", "config.node_db", "config.peer_port"} {
		assertStatus(t, report, id, model.StatusSkip)
	}
	assertStatus(t, report, "validator.config", model.StatusFail)
	for _, id := range []string{
		"validator.credentials", "validator.legacy_seed",
		"validator.config_mode", "validator.api_bind",
	} {
		assertStatus(t, report, id, model.StatusSkip)
	}
}

func TestValidatorCredentials(t *testing.T) {
	base := "[node_db]\ntype=NuDB\npath=/db\n"
	tests := []struct {
		name        string
		body        string
		credentials model.Status
		legacy      model.Status
	}{
		{"token only", base + "[validator_token]\nSUPER_SECRET_VALIDATOR_TOKEN_12345\n", model.StatusPass, model.StatusPass},
		{"seed only", base + "[validation_seed]\nSUPER_SECRET_VALIDATION_SEED_67890\n", model.StatusWarn, model.StatusWarn},
		{
			"both",
			base + "[validator_token]\nSUPER_SECRET_VALIDATOR_TOKEN_12345\n\n[validation_seed]\nSUPER_SECRET_VALIDATION_SEED_67890\n",
			model.StatusPass, model.StatusWarn,
		},
		{"neither", base, model.StatusFail, model.StatusPass},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: writeConfig(t, test.body)})
			assertStatus(t, report, "validator.credentials", test.credentials)
			assertStatus(t, report, "validator.legacy_seed", test.legacy)
			assertNoSecrets(t, report)
		})
	}
}

func TestValidatorConfigWithoutFile(t *testing.T) {
	report := run(newFakeProbe(), Options{Role: RoleValidator})
	check := assertStatus(t, report, "validator.config", model.StatusWarn)
	if !strings.Contains(check.Summary, "not provided") {
		t.Errorf("summary = %q", check.Summary)
	}
	for _, id := range []string{"validator.credentials", "validator.legacy_seed", "validator.config_mode", "validator.api_bind"} {
		assertStatus(t, report, id, model.StatusSkip)
	}
}

func TestConfigModeVerdict(t *testing.T) {
	tests := []struct {
		mode os.FileMode
		want bool
	}{
		{0o600, true},
		{0o400, true},
		{0o640, false},
		{0o644, false},
		{0o660, false},
		{0o664, false},
		{0o666, false},
		// Write-only: the owner cannot even read its own configuration.
		{0o200, false},
	}

	for _, test := range tests {
		if got := modeIsOwnerOnly(test.mode); got != test.want {
			t.Errorf("modeIsOwnerOnly(%#o) = %v, want %v", uint32(test.mode), got, test.want)
		}
	}
}

func TestValidatorConfigMode(t *testing.T) {
	tests := []struct {
		mode   os.FileMode
		status model.Status
	}{
		{0o600, model.StatusPass},
		{0o400, model.StatusPass},
		{0o640, model.StatusFail},
		{0o644, model.StatusFail},
		{0o660, model.StatusFail},
		{0o664, model.StatusFail},
		{0o666, model.StatusFail},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#o", uint32(test.mode)), func(t *testing.T) {
			path := writeConfig(t, "[node_db]\ntype=NuDB\npath=/db\n")
			if err := os.Chmod(path, test.mode); err != nil {
				t.Fatalf("Chmod() error = %v", err)
			}
			report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: path})
			assertStatus(t, report, "validator.config_mode", test.status)
		})
	}
}

// A configuration nobody can read fails as one primary error, and the checks
// that depend on its content are skipped instead of guessing.
func TestUnreadableConfigCascades(t *testing.T) {
	path := writeConfig(t, "[node_db]\ntype=NuDB\npath=/db\n")
	if err := os.Chmod(path, 0o200); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: path})
	assertStatus(t, report, "config.file", model.StatusFail)
	assertStatus(t, report, "validator.config", model.StatusFail)
	assertStatus(t, report, "validator.config_mode", model.StatusSkip)
}

func TestValidatorAPIBind(t *testing.T) {
	base := "[node_db]\ntype=NuDB\npath=/db\n[validator_token]\ntoken\n"
	tests := []struct {
		name   string
		body   string
		status model.Status
	}{
		{"loopback ipv4", base + "[server]\nport_rpc\n\n[port_rpc]\nip=127.0.0.1\nport=5005\nprotocol=http\n", model.StatusPass},
		{"loopback ipv6", base + "[server]\nport_ws\n\n[port_ws]\nip=::1\nport=6006\nprotocol=ws\n", model.StatusPass},
		{"wildcard ipv4", base + "[server]\nport_ws\n\n[port_ws]\nip=0.0.0.0\nport=6006\nprotocol=wss\n", model.StatusWarn},
		{"wildcard ipv6", base + "[server]\nport_ws\n\n[port_ws]\nip=::\nport=6006\nprotocol=wss\n", model.StatusWarn},
		{"private address", base + "[server]\nport_ws\n\n[port_ws]\nip=10.0.0.5\nport=6006\nprotocol=ws\n", model.StatusWarn},
		{"public address", base + "[server]\nport_ws\n\n[port_ws]\nip=203.0.113.7\nport=6006\nprotocol=https\n", model.StatusWarn},
		{"missing ip", base + "[server]\nport_ws\n\n[port_ws]\nport=6006\nprotocol=ws\n", model.StatusWarn},
		{"invalid ip", base + "[server]\nport_ws\n\n[port_ws]\nip=not-an-ip\nport=6006\nprotocol=ws\n", model.StatusWarn},
		{"inherited loopback", base + "[server]\nport_ws\nip=127.0.0.1\n\n[port_ws]\nport=6006\nprotocol=ws\n", model.StatusPass},
		{"inherited wildcard", base + "[server]\nport_ws\nip=0.0.0.0\n\n[port_ws]\nport=6006\nprotocol=ws\n", model.StatusWarn},
		{"peer only is ignored", base + "[server]\nport_peer\n\n[port_peer]\nip=0.0.0.0\nport=2459\nprotocol=peer\n", model.StatusPass},
		{"no listeners", base, model.StatusPass},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: writeConfig(t, test.body)})
			assertStatus(t, report, "validator.api_bind", test.status)
		})
	}
}

func TestValidatorGRPCBind(t *testing.T) {
	base := "[node_db]\ntype=NuDB\npath=/db\n[validator_token]\ntoken\n"
	tests := []struct {
		name   string
		body   string
		status model.Status
	}{
		{"no grpc section", base, model.StatusPass},
		{"loopback", base + "[port_grpc]\nip=127.0.0.1\nport=50051\n", model.StatusPass},
		{"wildcard ipv4", base + "[port_grpc]\nip=0.0.0.0\nport=50051\n", model.StatusWarn},
		{"wildcard ipv6", base + "[port_grpc]\nip=::\nport=50051\n", model.StatusWarn},
		{"omitted ip uses the loopback default", base + "[port_grpc]\nport=50051\n", model.StatusPass},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: writeConfig(t, test.body)})
			assertStatus(t, report, "validator.api_bind", test.status)
		})
	}
}

func TestExposureWordingIsNotProof(t *testing.T) {
	body := "[node_db]\ntype=NuDB\npath=/db\n[server]\nport_ws\n\n[port_ws]\nip=0.0.0.0\nport=6006\nprotocol=wss\n"
	report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: writeConfig(t, body)})
	check := find(t, report, "validator.api_bind")
	for _, forbidden := range []string{"public Internet", "reachable", "exposed to the Internet"} {
		if strings.Contains(check.Summary, forbidden) {
			t.Errorf("summary = %q, must not claim %q", check.Summary, forbidden)
		}
	}
}

func TestSecretsNeverReachTheReport(t *testing.T) {
	for _, name := range []string{"validator-token.cfg", "validator-legacy.cfg", "validator-public-api.cfg", "validator-grpc-public.cfg"} {
		t.Run(name, func(t *testing.T) {
			report := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: testdata(t, name)})
			assertNoSecrets(t, report)
		})
	}
}

func TestOverallResults(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		p := newFakeProbe()
		report := run(p, Options{ConfigPath: writeConfig(t,
			"[node_db]\ntype=NuDB\npath=/db\n[node_size]\nhuge\n[server]\nport_peer\n\n[port_peer]\nport=2459\nprotocol=peer\n"),
			DataDir: "/srv/db"})
		if report.Result != model.ResultReady {
			t.Fatalf("result = %s, checks = %+v", report.Result, report.Checks)
		}
	})

	t.Run("warnings only", func(t *testing.T) {
		p := newFakeProbe()
		p.storage.Media = probe.MediaUnknown
		report := run(p, Options{DataDir: "/srv/db"})
		if report.Result != model.ResultReadyWithWarnings {
			t.Fatalf("result = %s", report.Result)
		}
	})

	t.Run("failure", func(t *testing.T) {
		p := newFakeProbe()
		p.cpu = 2
		report := run(p, Options{DataDir: "/srv/db"})
		if report.Result != model.ResultNotReady {
			t.Fatalf("result = %s", report.Result)
		}
	})

	t.Run("skips alone do not downgrade", func(t *testing.T) {
		p := newFakeProbe()
		report := run(p, Options{DataDir: "/srv/db"})
		if report.Summary.Skip == 0 {
			t.Fatal("expected skipped config checks")
		}
		if report.Result != model.ResultReady {
			t.Fatalf("result = %s, checks = %+v", report.Result, report.Checks)
		}
	})
}

func TestReportIsDeterministic(t *testing.T) {
	config := testdata(t, "validator-public-api.cfg")
	first := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: config})
	second := run(newFakeProbe(), Options{Role: RoleValidator, ConfigPath: config})

	if len(first.Checks) != len(second.Checks) {
		t.Fatalf("check counts differ: %d and %d", len(first.Checks), len(second.Checks))
	}
	for index := range first.Checks {
		if first.Checks[index] != second.Checks[index] {
			t.Errorf("check %d differs: %+v and %+v", index, first.Checks[index], second.Checks[index])
		}
	}
}

func TestPathsFromConfigAreQuoted(t *testing.T) {
	path := writeConfig(t, "[node_db]\ntype=NuDB\npath=/var/lib/xrpld/db\n")
	report := run(newFakeProbe(), Options{ConfigPath: path})
	check := find(t, report, "config.node_db")
	if !strings.Contains(check.Summary, `"/var/lib/xrpld/db"`) {
		t.Errorf("summary = %q, want a quoted path", check.Summary)
	}
}

// writeConfig writes a configuration file for one test.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "xrpld.cfg")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func nodeSizeSection(value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("[node_size]\n%s\n", value)
}

// assertNoSecrets fails when a fixture secret appears anywhere in a report.
func assertNoSecrets(t *testing.T, report model.Report) {
	t.Helper()
	var builder strings.Builder
	for _, check := range report.Checks {
		builder.WriteString(check.ID)
		builder.WriteString(check.Summary)
		builder.WriteString(check.Observed)
		builder.WriteString(check.Expected)
	}
	for _, secret := range []string{"SUPER_SECRET_VALIDATOR_TOKEN_12345", "SUPER_SECRET_VALIDATION_SEED_67890"} {
		if strings.Contains(builder.String(), secret) {
			t.Errorf("report leaked %q", secret)
		}
	}
}
