package preflight

import (
	"context"
	"fmt"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/model"
	"github.com/LionelTrichet/xrpl-node-preflight/internal/probe"
)

// requiredNofile is the file-descriptor limit xrpld troubleshooting guidance
// asks for.
const requiredNofile = 65536

// nofileCheck judges the file-descriptor limit. Only a limit read from the
// xrpld service is authoritative; the limit of this process says nothing
// about the service and can never produce a failure.
func nofileCheck(ctx context.Context, p probe.Probe, level string) model.Check {
	expected := fmt.Sprintf(">= %d for xrpld.service", requiredNofile)

	info, err := p.Nofile(ctx)
	if err != nil {
		return check("limits.nofile", model.StatusWarn,
			"xrpld service limit could not be verified", "", expected)
	}

	if info.ServiceVerified {
		if info.ServiceInfinity {
			return check("limits.nofile", model.StatusPass,
				"xrpld.service LimitNOFILE is unlimited", "infinity", expected)
		}

		observed := fmt.Sprintf("%d", info.ServiceLimit)
		if info.ServiceLimit >= requiredNofile {
			return check("limits.nofile", model.StatusPass,
				fmt.Sprintf("xrpld.service LimitNOFILE is %d", info.ServiceLimit), observed, expected)
		}

		status := model.StatusFail
		if level == LevelMinimum {
			status = model.StatusWarn
		}
		return check("limits.nofile", status,
			fmt.Sprintf("xrpld.service LimitNOFILE is %d, below %d", info.ServiceLimit, requiredNofile),
			observed, expected)
	}

	// Without the service limit the current process limit is the only number
	// available, and it does not predict what xrpld.service will receive.
	if info.ProcessKnown {
		return check("limits.nofile", model.StatusWarn,
			fmt.Sprintf("xrpld service limit could not be verified; current process hard limit is %d", info.ProcessLimit),
			fmt.Sprintf("process %d", info.ProcessLimit), expected)
	}
	return check("limits.nofile", model.StatusWarn,
		"xrpld service limit could not be verified", "", expected)
}
