package probe

import (
	"errors"
	"strconv"
	"strings"
)

// rtfUp is the RTF_UP flag of a routing table entry.
const rtfUp = 0x1

// errNoDefaultRoute is returned when the routing table has no usable IPv4
// default route.
var errNoDefaultRoute = errors.New("no IPv4 default route")

// parseDefaultRoute returns the interface of the IPv4 default route with the
// lowest metric. Policy routing is out of scope: only the main table shown in
// /proc/net/route is considered.
func parseDefaultRoute(data string) (string, error) {
	best := ""
	bestMetric := 0

	for index, line := range strings.Split(data, "\n") {
		if index == 0 {
			// Header line.
			continue
		}
		fields := strings.Fields(line)
		// 0: iface, 1: destination, 2: gateway, 3: flags, ... 6: metric.
		if len(fields) < 7 {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}

		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&rtfUp == 0 {
			continue
		}
		metric, err := strconv.Atoi(fields[6])
		if err != nil {
			continue
		}

		if best == "" || metric < bestMetric {
			best = fields[0]
			bestMetric = metric
		}
	}

	if best == "" {
		return "", errNoDefaultRoute
	}
	return best, nil
}
