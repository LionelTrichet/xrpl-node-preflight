// Package xrpld extracts the few facts the readiness checks need from an
// xrpld configuration file. It is not a validator for the configuration
// format, and it never copies credential values out of the source buffer.
package xrpld

// PortSettings holds the only listener settings this tool uses. Users,
// passwords, TLS material and every other listener setting are ignored on
// purpose, so they are never retained.
type PortSettings struct {
	IP       string
	Port     string
	Protocol string
}

// ConfigFacts is everything the checks may learn from a configuration file.
// Credentials appear only as booleans: no token or seed text is stored here,
// in errors or in reports.
type ConfigFacts struct {
	NodeSize string

	NodeDBType string
	NodeDBPath string

	// ServerPortNames are the listener sections activated by [server], in
	// file order and normalized for lookup.
	ServerPortNames []string
	ServerDefaults  PortSettings
	PortSections    map[string]PortSettings

	GRPCPresent bool
	GRPC        PortSettings

	ValidatorTokenPresent bool
	ValidationSeedPresent bool
}
