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

// nofileCheck judges the file-descriptor limit. Only the soft limit read from
// the xrpld service is authoritative: xrpld starts with it and raises it only
// to its own minimum estimate, so a high hard limit alone does not reach the
// recommended value. The limit of this process says nothing about the service
// and can never produce a failure.
func nofileCheck(ctx context.Context, p probe.Probe, level string) model.Check {
	expected := fmt.Sprintf(">= %d for xrpld.service", requiredNofile)

	info, err := p.Nofile(ctx)
	if err != nil {
		return check("limits.nofile", model.StatusWarn,
			"xrpld service limit could not be verified", "", expected)
	}

	if info.ServiceVerified {
		if info.ServiceSoftInfinity {
			return check("limits.nofile", model.StatusPass,
				"xrpld.service soft LimitNOFILE is unlimited", "infinity", expected)
		}

		observed := fmt.Sprintf("%d", info.ServiceSoftLimit)
		if info.ServiceSoftLimit >= requiredNofile {
			return check("limits.nofile", model.StatusPass,
				fmt.Sprintf("xrpld.service soft LimitNOFILE is %d", info.ServiceSoftLimit), observed, expected)
		}

		status := model.StatusFail
		if level == LevelMinimum {
			status = model.StatusWarn
		}
		return check("limits.nofile", status,
			fmt.Sprintf("xrpld.service soft LimitNOFILE is %d, below %d", info.ServiceSoftLimit, requiredNofile),
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
