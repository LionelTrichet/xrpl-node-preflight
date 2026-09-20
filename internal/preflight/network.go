package preflight

import (
	"context"
	"fmt"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
)

// requiredLinkSpeed is the recommended interface speed in Mbit/s.
const requiredLinkSpeed = 1000

// networkCheck reports the link state and speed of the selected interface.
// Addresses are never reported: only the interface name, its state and its
// speed appear in the result.
func networkCheck(p probe.Probe, opts Options) model.Check {
	expected := fmt.Sprintf("interface up, >= %d Mbit/s", requiredLinkSpeed)

	name := opts.Interface
	explicit := name != ""
	if !explicit {
		detected, err := p.DefaultInterface()
		if err != nil {
			return check("network.interface", model.StatusWarn,
				"no IPv4 default route found; network interface could not be determined", "", expected)
		}
		name = detected
	}

	info, err := p.NetworkInterface(name)
	if err != nil {
		if explicit {
			return check("network.interface", model.StatusFail,
				fmt.Sprintf("interface %s does not exist", sanitize(name)), sanitize(name), expected)
		}
		return check("network.interface", model.StatusWarn,
			fmt.Sprintf("interface %s could not be inspected", sanitize(name)), sanitize(name), expected)
	}

	status := model.StatusPass
	summary := ""

	switch info.OperState {
	case "up":
	case "down", "dormant", "lowerlayerdown", "notpresent":
		status = worst(status, model.StatusFail)
		summary = fmt.Sprintf("%s is %s", info.Name, info.OperState)
	default:
		status = worst(status, model.StatusWarn)
		summary = fmt.Sprintf("%s link state is unknown", info.Name)
	}

	speedStatus, speedText := linkSpeedVerdict(info, opts.Level)
	status = worst(status, speedStatus)
	if summary == "" {
		summary = fmt.Sprintf("%s, %s", info.Name, speedText)
	} else {
		summary = fmt.Sprintf("%s, %s", summary, speedText)
	}

	observed := fmt.Sprintf("%s, state %s, %s", info.Name, displayState(info.OperState), speedText)
	return check("network.interface", status, summary, observed, expected)
}

func linkSpeedVerdict(info probe.NetworkInfo, level string) (model.Status, string) {
	if info.SpeedMbit <= 0 {
		return model.StatusWarn, "link speed unknown"
	}

	text := fmt.Sprintf("%d Mbit/s", info.SpeedMbit)
	if info.SpeedMbit >= requiredLinkSpeed {
		return model.StatusPass, text
	}
	if level == LevelMinimum {
		return model.StatusWarn, text
	}
	return model.StatusFail, text
}

func displayState(state string) string {
	if state == "" {
		return "unknown"
	}
	return sanitize(state)
}

// timeCheck reports the local clock synchronization state. No time server is
// ever contacted: only the local daemon is asked.
func timeCheck(ctx context.Context, p probe.Probe, level string) model.Check {
	expected := "synchronized system clock"

	info, err := p.TimeSync(ctx)
	if err != nil {
		return check("time.sync", model.StatusWarn,
			"time synchronization state could not be determined", probe.TimeUnknown, expected)
	}

	source := ""
	if info.Source != "" {
		source = fmt.Sprintf(" (%s)", sanitize(info.Source))
	}

	switch info.State {
	case probe.TimeSynchronized:
		return check("time.sync", model.StatusPass,
			fmt.Sprintf("system clock is synchronized%s", source), info.State, expected)
	case probe.TimeUnsynchronized:
		status := model.StatusFail
		if level == LevelMinimum {
			status = model.StatusWarn
		}
		return check("time.sync", status,
			fmt.Sprintf("system clock is not synchronized%s", source), info.State, expected)
	default:
		return check("time.sync", model.StatusWarn,
			"time synchronization state could not be determined", probe.TimeUnknown, expected)
	}
}
