package xrpld

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	tokenSentinel = "SUPER_SECRET_VALIDATOR_TOKEN_12345"
	seedSentinel  = "SUPER_SECRET_VALIDATION_SEED_67890"
)

func parseFile(t *testing.T, name string) ConfigFacts {
	t.Helper()
	facts, err := ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", name, err)
	}
	return facts
}

func parseString(t *testing.T, text string) ConfigFacts {
	t.Helper()
	facts, err := Parse([]byte(text))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return facts
}

func TestNodeConfig(t *testing.T) {
	facts := parseFile(t, "node.cfg")

	if facts.NodeSize != "huge" {
		t.Errorf("NodeSize = %q, want %q", facts.NodeSize, "huge")
	}
	if facts.NodeDBType != "NuDB" || facts.NodeDBPath != "/var/lib/xrpld/db/nudb" {
		t.Errorf("NodeDB = %q %q", facts.NodeDBType, facts.NodeDBPath)
	}
	if len(facts.ServerPortNames) != 3 {
		t.Fatalf("ServerPortNames = %v, want three listeners", facts.ServerPortNames)
	}
	if facts.GRPCPresent {
		t.Error("GRPCPresent = true, want false")
	}
}

func TestServerSectionIsMixed(t *testing.T) {
	facts := parseFile(t, "server-defaults.cfg")

	if got := facts.ServerPortNames; len(got) != 1 || got[0] != "port_public" {
		t.Fatalf("ServerPortNames = %v, want only port_public", got)
	}
	if facts.ServerDefaults.IP != "0.0.0.0" {
		t.Errorf("server default ip = %q", facts.ServerDefaults.IP)
	}
}

// The inheritance rule is the reason [server] exists: defaults apply unless
// the port section overrides them.
func TestEffectiveListenerInheritsServerDefaults(t *testing.T) {
	facts := parseFile(t, "server-defaults.cfg")

	listeners := facts.Listeners()
	if len(listeners) != 1 {
		t.Fatalf("Listeners() = %v", listeners)
	}
	listener := listeners[0]
	if listener.IP != "0.0.0.0" {
		t.Errorf("ip = %q, want the inherited 0.0.0.0", listener.IP)
	}
	if listener.Port != "5005" {
		t.Errorf("port = %q, want 5005", listener.Port)
	}
	if !listener.HasProtocol("http") {
		t.Errorf("protocols = %v, want http", listener.Protocols)
	}
}

func TestPortOverridesEveryServerDefault(t *testing.T) {
	facts := parseString(t, `[server]
port_public
ip = 0.0.0.0
port = 1111
protocol = http

[port_public]
ip = 127.0.0.1
port = 5005
protocol = ws
`)

	listener := facts.Listeners()[0]
	if listener.IP != "127.0.0.1" || listener.Port != "5005" || !listener.HasProtocol("ws") {
		t.Fatalf("listener = %+v, want the port section to win", listener)
	}
	if listener.HasProtocol("http") {
		t.Error("protocol http survived the override")
	}
}

func TestIdentifiersAreCaseInsensitive(t *testing.T) {
	facts := parseString(t, `[SERVER]
Port_Public

[Port_Public]
IP = 127.0.0.1
PORT = 5005
PROTOCOL = HTTP

[NODE_DB]
TYPE=NuDB
PATH=/var/lib/xrpld/db

[Node_Size]
HUGE
`)

	listener := facts.Listeners()[0]
	if listener.IP != "127.0.0.1" || listener.Port != "5005" {
		t.Fatalf("listener = %+v", listener)
	}
	if !listener.HasProtocol("http") {
		t.Errorf("protocols = %v, want the token compared without case", listener.Protocols)
	}
	if facts.NodeDBType != "NuDB" || facts.NodeDBPath != "/var/lib/xrpld/db" {
		t.Errorf("NodeDB = %q %q, want values kept as written", facts.NodeDBType, facts.NodeDBPath)
	}
	// Values are not lowercased globally.
	if facts.NodeSize != "HUGE" {
		t.Errorf("NodeSize = %q, want the value as written", facts.NodeSize)
	}
}

func TestLineEndingsAreEquivalent(t *testing.T) {
	unix := `[node_db]
type=NuDB
path=/db
`
	dos := strings.ReplaceAll(unix, "\n", "\r\n")
	mac := strings.ReplaceAll(unix, "\n", "\r")

	first := parseString(t, unix)
	for name, text := range map[string]string{"CRLF": dos, "CR": mac} {
		facts := parseString(t, text)
		if facts.NodeDBType != first.NodeDBType || facts.NodeDBPath != first.NodeDBPath {
			t.Errorf("%s: facts = %+v, want the same as LF", name, facts)
		}
	}
}

func TestBlankLinesAndComments(t *testing.T) {
	facts := parseString(t, `
   # a comment

[node_db]
   # another comment
type=NuDB

path=/db
`)
	if facts.NodeDBType != "NuDB" || facts.NodeDBPath != "/db" {
		t.Fatalf("facts = %+v", facts)
	}
}

func TestMalformedSectionHeader(t *testing.T) {
	if _, err := ReadFile(filepath.Join("testdata", "malformed.cfg")); err == nil {
		t.Fatal("ReadFile() error = nil, want a malformed section error")
	}
	for _, text := range []string{"[1port]\n", "[port peer]\n", "[port_peer\n", "[]\n"} {
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("Parse(%q) error = nil, want error", text)
		}
	}
}

func TestInvalidUTF8(t *testing.T) {
	if _, err := Parse([]byte{'[', 0xff, 0xfe, ']'}); err == nil {
		t.Fatal("Parse() error = nil, want an encoding error")
	}
}

func TestOversizedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.cfg")
	data := append([]byte("[node_db]\n"), make([]byte, MaxConfigBytes)...)
	for i := range data[10:] {
		data[10+i] = '#'
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := ReadFile(path); err == nil {
		t.Fatal("ReadFile() error = nil, want the size limit")
	}
}

func TestNonRegularFile(t *testing.T) {
	if _, err := ReadFile(t.TempDir()); err == nil {
		t.Fatal("ReadFile() error = nil, want a regular-file error")
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := ReadFile(filepath.Join(t.TempDir(), "absent.cfg")); err == nil {
		t.Fatal("ReadFile() error = nil, want error")
	}
}

func TestCredentialsAreOnlyBooleans(t *testing.T) {
	token := parseFile(t, "validator-token.cfg")
	if !token.ValidatorTokenPresent || token.ValidationSeedPresent {
		t.Fatalf("token config = %+v", token)
	}

	legacy := parseFile(t, "validator-legacy.cfg")
	if legacy.ValidatorTokenPresent || !legacy.ValidationSeedPresent {
		t.Fatalf("legacy config = %+v", legacy)
	}

	// No secret text may survive anywhere in the extracted facts.
	for name, facts := range map[string]ConfigFacts{"token": token, "legacy": legacy} {
		rendered := renderFacts(facts)
		for _, secret := range []string{tokenSentinel, seedSentinel} {
			if strings.Contains(rendered, secret) {
				t.Errorf("%s config leaked %q into the facts", name, secret)
			}
		}
	}
}

func TestPortSectionsKeepOnlyListenerSettings(t *testing.T) {
	facts := parseString(t, `[server]
port_rpc

[port_rpc]
ip = 127.0.0.1
port = 5005
protocol = http
admin_user = admin
admin_password = SUPER_SECRET_VALIDATOR_TOKEN_12345
ssl_key = /etc/ssl/private/key.pem
secure_gateway = 10.0.0.1
`)

	if strings.Contains(renderFacts(facts), tokenSentinel) {
		t.Fatal("a listener credential was retained")
	}
	settings := facts.PortSections["port_rpc"]
	if settings.IP != "127.0.0.1" || settings.Port != "5005" || settings.Protocol != "http" {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestGRPCSection(t *testing.T) {
	facts := parseFile(t, "validator-grpc-public.cfg")
	listener, ok := facts.GRPCListener()
	if !ok {
		t.Fatal("GRPCListener() not found")
	}
	if listener.IP != "0.0.0.0" || listener.Port != "50051" {
		t.Fatalf("gRPC listener = %+v", listener)
	}

	without := parseFile(t, "node.cfg")
	if _, ok := without.GRPCListener(); ok {
		t.Error("GRPCListener() found without a [port_grpc] section")
	}

	omitted := parseString(t, "[port_grpc]\nport = 50051\n")
	grpc, ok := omitted.GRPCListener()
	if !ok || grpc.IP != "" {
		t.Fatalf("gRPC listener = %+v, want an omitted ip", grpc)
	}
}

func TestProtocolTokens(t *testing.T) {
	facts := parseString(t, `[server]
port_mixed

[port_mixed]
port = 2459
protocol = Peer, HTTPS
`)
	listener := facts.Listeners()[0]
	if !listener.HasProtocol("peer") || !listener.HasProtocol("https") {
		t.Fatalf("protocols = %v", listener.Protocols)
	}
	if ProtocolTokens("  ") != nil {
		t.Error("ProtocolTokens(blank) should be nil")
	}
}

// renderFacts flattens the facts into text so a test can look for secrets.
func renderFacts(facts ConfigFacts) string {
	var builder strings.Builder
	builder.WriteString(facts.NodeSize)
	builder.WriteString(facts.NodeDBType)
	builder.WriteString(facts.NodeDBPath)
	builder.WriteString(strings.Join(facts.ServerPortNames, " "))
	builder.WriteString(facts.ServerDefaults.IP + facts.ServerDefaults.Port + facts.ServerDefaults.Protocol)
	builder.WriteString(facts.GRPC.IP + facts.GRPC.Port + facts.GRPC.Protocol)
	for name, settings := range facts.PortSections {
		builder.WriteString(name + settings.IP + settings.Port + settings.Protocol)
	}
	return builder.String()
}
