package probe

import (
	"strings"
)

// mountEntry is the part of a /proc/self/mountinfo line this tool needs.
type mountEntry struct {
	// MountPoint is the decoded mount point path.
	MountPoint string
	// Device is the "major:minor" identifier of the backing device.
	Device string
}

// parseMountinfo reads the mount table. Lines that do not have the expected
// shape are ignored rather than failing the whole probe.
func parseMountinfo(data string) []mountEntry {
	var entries []mountEntry
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		// 0: mount id, 1: parent id, 2: major:minor, 3: root, 4: mount point.
		if len(fields) < 5 {
			continue
		}
		if !strings.Contains(fields[2], ":") {
			continue
		}
		entries = append(entries, mountEntry{
			MountPoint: decodeMountEscapes(fields[4]),
			Device:     fields[2],
		})
	}
	return entries
}

// findMount returns the entry whose mount point contains path, preferring the
// longest match. Matching is by path component, so "/var/lib" never matches
// "/var/library".
func findMount(entries []mountEntry, path string) (mountEntry, bool) {
	var best mountEntry
	found := false
	for _, entry := range entries {
		if !containsPath(entry.MountPoint, path) {
			continue
		}
		if !found || len(entry.MountPoint) > len(best.MountPoint) {
			best = entry
			found = true
		}
	}
	return best, found
}

func containsPath(mountPoint, path string) bool {
	if mountPoint == "/" {
		return strings.HasPrefix(path, "/")
	}
	if path == mountPoint {
		return true
	}
	return strings.HasPrefix(path, mountPoint+"/")
}

// decodeMountEscapes decodes the octal escapes the kernel writes for
// characters that would otherwise break the field layout.
func decodeMountEscapes(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}

	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); {
		if value[i] == '\\' && i+3 < len(value) {
			switch value[i+1 : i+4] {
			case "040":
				builder.WriteByte(' ')
				i += 4
				continue
			case "011":
				builder.WriteByte('\t')
				i += 4
				continue
			case "012":
				builder.WriteByte('\n')
				i += 4
				continue
			case "134":
				builder.WriteByte('\\')
				i += 4
				continue
			}
		}
		builder.WriteByte(value[i])
		i++
	}
	return builder.String()
}
