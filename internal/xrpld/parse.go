package xrpld

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxConfigBytes is the largest configuration file accepted, 1 MiB.
const MaxConfigBytes = 1048576

var sectionPattern = regexp.MustCompile(`^\[([A-Za-z][A-Za-z0-9_]*)\]$`)

// ReadFile reads a configuration file within the size limit and extracts the
// facts the checks need. The file must be a regular file and valid UTF-8.
func ReadFile(path string) (ConfigFacts, error) {
	file, err := os.Open(path)
	if err != nil {
		return ConfigFacts{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return ConfigFacts{}, fmt.Errorf("stat config: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return ConfigFacts{}, errors.New("config is not a regular file")
	}
	if stat.Size() > MaxConfigBytes {
		return ConfigFacts{}, fmt.Errorf("config exceeds %d bytes", MaxConfigBytes)
	}

	// One byte beyond the limit is read so that a file growing between the
	// stat and the read is rejected rather than silently truncated.
	data, err := io.ReadAll(io.LimitReader(file, MaxConfigBytes+1))
	if err != nil {
		return ConfigFacts{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) > MaxConfigBytes {
		return ConfigFacts{}, fmt.Errorf("config exceeds %d bytes", MaxConfigBytes)
	}
	return Parse(data)
}

// Parse extracts the facts from configuration bytes. Section names, keys and
// known identifiers are compared without case; values are kept as written.
//
// The buffer inevitably contains any configured validator secret while it is
// being parsed, but no secret value is copied into the returned facts, into
// an error or anywhere else.
func Parse(data []byte) (ConfigFacts, error) {
	if !utf8.Valid(data) {
		return ConfigFacts{}, errors.New("config is not valid UTF-8")
	}

	facts := ConfigFacts{PortSections: map[string]PortSettings{}}
	section := ""

	for _, line := range strings.Split(normalizeLineEndings(string(data)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "[") {
			match := sectionPattern.FindStringSubmatch(trimmed)
			if match == nil {
				return ConfigFacts{}, fmt.Errorf("malformed section header on a configuration line")
			}
			section = strings.ToLower(match[1])
			if section == "port_grpc" {
				facts.GRPCPresent = true
			}
			continue
		}

		switch section {
		case "":
			// Content before the first section header is not used.
		case "server":
			parseServerLine(&facts, trimmed)
		case "node_size":
			if facts.NodeSize == "" {
				facts.NodeSize = trimmed
			}
		case "node_db":
			parseNodeDBLine(&facts, trimmed)
		case "validator_token":
			facts.ValidatorTokenPresent = true
		case "validation_seed":
			facts.ValidationSeedPresent = true
		case "port_grpc":
			applyPortKey(&facts.GRPC, trimmed)
		default:
			settings := facts.PortSections[section]
			applyPortKey(&settings, trimmed)
			facts.PortSections[section] = settings
		}
	}

	return facts, nil
}

// parseServerLine handles the mixed [server] section: a line with a "=" is a
// default inherited by listeners, and a bare line names an active listener.
func parseServerLine(facts *ConfigFacts, line string) {
	if strings.Contains(line, "=") {
		applyPortKey(&facts.ServerDefaults, line)
		return
	}
	facts.ServerPortNames = append(facts.ServerPortNames, strings.ToLower(line))
}

func parseNodeDBLine(facts *ConfigFacts, line string) {
	key, value, found := strings.Cut(line, "=")
	if !found {
		return
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "type":
		facts.NodeDBType = strings.TrimSpace(value)
	case "path":
		facts.NodeDBPath = strings.TrimSpace(value)
	}
}

// applyPortKey stores the three listener settings this tool uses and ignores
// every other key, including credentials and TLS material.
func applyPortKey(settings *PortSettings, line string) {
	key, value, found := strings.Cut(line, "=")
	if !found {
		return
	}
	value = strings.TrimSpace(value)
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "ip":
		settings.IP = value
	case "port":
		settings.Port = value
	case "protocol":
		settings.Protocol = value
	}
}

// normalizeLineEndings accepts DOS, UNIX and classic Mac line endings, as the
// upstream configuration format does.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}
