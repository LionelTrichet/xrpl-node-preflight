package preflight

import (
	"fmt"
	"net"
	"os"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/xrpld"
)

// grpcDefaultIP is the documented default bind address of [port_grpc].
const grpcDefaultIP = "127.0.0.1"

func validatorConfigCheck(config configState) model.Check {
	switch {
	case !config.supplied:
		return check("validator.config", model.StatusWarn,
			"validator configuration was not provided; validator-specific checks are incomplete",
			"", "an xrpld.cfg")
	case !config.ok:
		return check("validator.config", model.StatusFail,
			"validator configuration could not be read", quotePath(config.path), "a readable xrpld.cfg")
	default:
		return check("validator.config", model.StatusPass,
			"validator configuration read", quotePath(config.path), "")
	}
}

// validatorCredentialsCheck reports which credential configures validation.
// Only the presence of the sections is known here: no token or seed value is
// ever read out of the parser.
func validatorCredentialsCheck(config configState) model.Check {
	if !config.ok {
		return skipped("validator.credentials", configSkipReason(config))
	}

	facts := config.facts
	switch {
	case facts.ValidatorTokenPresent:
		return check("validator.credentials", model.StatusPass,
			"validator token present", "validator_token", "validator_token")
	case facts.ValidationSeedPresent:
		return check("validator.credentials", model.StatusWarn,
			"validation is configured through the legacy validation_seed", "validation_seed", "validator_token")
	default:
		return check("validator.credentials", model.StatusFail,
			"no validator credential configured", "none", "validator_token")
	}
}

func validatorLegacySeedCheck(config configState) model.Check {
	if !config.ok {
		return skipped("validator.legacy_seed", configSkipReason(config))
	}

	if config.facts.ValidationSeedPresent {
		return check("validator.legacy_seed", model.StatusWarn,
			"legacy validation_seed is present; a validator_token generated with validator-keys is recommended",
			"validation_seed present", "no validation_seed")
	}
	return check("validator.legacy_seed", model.StatusPass,
		"no legacy validation_seed", "validation_seed absent", "no validation_seed")
}

// validatorConfigModeCheck looks at the POSIX mode bits of the configuration
// file. It says nothing about ownership, ACLs or mandatory access control.
func validatorConfigModeCheck(config configState) model.Check {
	if !config.ok {
		return skipped("validator.config_mode", configSkipReason(config))
	}

	info, err := os.Stat(config.path)
	if err != nil {
		return check("validator.config_mode", model.StatusWarn,
			"configuration mode could not be read", "", "owner-readable, no group or other bits")
	}

	mode := info.Mode().Perm()
	observed := fmt.Sprintf("%#o", uint32(mode))
	expected := "owner-readable, no group or other bits"

	if modeIsOwnerOnly(mode) {
		return check("validator.config_mode", model.StatusPass,
			"owner-readable; no group/other mode bits", observed, expected)
	}
	return check("validator.config_mode", model.StatusFail,
		fmt.Sprintf("configuration mode %s exposes the file beyond its owner", observed), observed, expected)
}

// modeIsOwnerOnly reports whether the POSIX mode bits keep the file readable
// by its owner and closed to group and other.
func modeIsOwnerOnly(mode os.FileMode) bool {
	return mode&0o400 != 0 && mode&0o077 == 0
}

// validatorAPIBindCheck reviews where the local API and gRPC listeners bind.
// A non-loopback bind is a prompt for operator review, not evidence that the
// listener is reachable from the Internet.
func validatorAPIBindCheck(config configState) model.Check {
	if !config.ok {
		return skipped("validator.api_bind", configSkipReason(config))
	}

	status := model.StatusPass
	exposed := []string{}
	unclassified := []string{}
	total := 0

	for _, listener := range config.facts.Listeners() {
		if !isAPIListener(listener) {
			continue
		}
		total++
		switch classifyBind(listener.IP) {
		case bindLoopback:
		case bindExposed:
			status = worst(status, model.StatusWarn)
			exposed = append(exposed, sanitize(listener.Name))
		default:
			status = worst(status, model.StatusWarn)
			unclassified = append(unclassified, sanitize(listener.Name))
		}
	}

	if grpc, ok := config.facts.GRPCListener(); ok {
		total++
		ip := grpc.IP
		if ip == "" {
			// The documented default is loopback; a public bind is never
			// assumed.
			ip = grpcDefaultIP
		}
		switch classifyBind(ip) {
		case bindLoopback:
		case bindExposed:
			status = worst(status, model.StatusWarn)
			exposed = append(exposed, "port_grpc")
		default:
			status = worst(status, model.StatusWarn)
			unclassified = append(unclassified, "port_grpc")
		}
	}

	observed := fmt.Sprintf("%d API listeners", total)
	expected := "loopback API listeners"

	switch {
	case status == model.StatusPass && total == 0:
		return check("validator.api_bind", model.StatusPass,
			"no API or gRPC listeners configured", observed, expected)
	case status == model.StatusPass:
		return check("validator.api_bind", model.StatusPass,
			"all API listeners bind loopback addresses", observed, expected)
	case len(exposed) > 0:
		return check("validator.api_bind", model.StatusWarn,
			"validator API binds to a non-loopback interface; verify firewall and network exposure",
			fmt.Sprintf("non-loopback: %s", joinNames(exposed)), expected)
	default:
		return check("validator.api_bind", model.StatusWarn,
			"validator API bind address could not be classified; verify firewall and network exposure",
			fmt.Sprintf("unclassified: %s", joinNames(unclassified)), expected)
	}
}

func isAPIListener(listener xrpld.Listener) bool {
	for _, protocol := range []string{"http", "https", "ws", "wss"} {
		if listener.HasProtocol(protocol) {
			return true
		}
	}
	return false
}

// Bind classifications.
const (
	bindLoopback = iota
	bindExposed
	bindUnknown
)

func classifyBind(value string) int {
	ip := net.ParseIP(value)
	if ip == nil {
		return bindUnknown
	}
	if ip.IsLoopback() {
		return bindLoopback
	}
	return bindExposed
}

func joinNames(names []string) string {
	result := ""
	for index, name := range names {
		if index > 0 {
			result += ", "
		}
		result += name
	}
	return result
}
