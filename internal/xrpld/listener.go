package xrpld

import "strings"

// Listener is one active universal port with its effective settings, after
// the [server] defaults have been applied and port-specific values have
// overridden them.
type Listener struct {
	Name      string
	IP        string
	Port      string
	Protocols []string
}

// Listeners returns the active listeners named in [server], in file order.
func (f ConfigFacts) Listeners() []Listener {
	listeners := make([]Listener, 0, len(f.ServerPortNames))
	for _, name := range f.ServerPortNames {
		settings := effective(f.ServerDefaults, f.PortSections[name])
		listeners = append(listeners, Listener{
			Name:      name,
			IP:        settings.IP,
			Port:      settings.Port,
			Protocols: ProtocolTokens(settings.Protocol),
		})
	}
	return listeners
}

// GRPCListener returns the gRPC listener when [port_grpc] is configured. The
// stanza stands on its own: it neither appears in [server] nor declares a
// protocol.
func (f ConfigFacts) GRPCListener() (Listener, bool) {
	if !f.GRPCPresent {
		return Listener{}, false
	}
	return Listener{Name: "port_grpc", IP: f.GRPC.IP, Port: f.GRPC.Port}, true
}

// HasProtocol reports whether the listener serves the given protocol token.
func (l Listener) HasProtocol(token string) bool {
	for _, protocol := range l.Protocols {
		if protocol == token {
			return true
		}
	}
	return false
}

// ProtocolTokens splits a protocol setting into lowercase tokens.
func ProtocolTokens(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.ToLower(strings.TrimSpace(part))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// effective applies port-specific overrides on top of the server defaults.
func effective(defaults, override PortSettings) PortSettings {
	settings := defaults
	if override.IP != "" {
		settings.IP = override.IP
	}
	if override.Port != "" {
		settings.Port = override.Port
	}
	if override.Protocol != "" {
		settings.Protocol = override.Protocol
	}
	return settings
}
