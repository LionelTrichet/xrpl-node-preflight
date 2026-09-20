# Architecture

## Purpose

Answer one question locally and deterministically: is this Linux host, with this optional `xrpld` configuration, ready to run an XRP Ledger node or validator?

## System Boundary

Inside: reading local Linux state, reading a supplied configuration, deciding each check, rendering a report.

Outside: installing or running `xrpld`, changing the host, monitoring, benchmarking, inspecting live consensus, and anything that leaves the machine.

## CLI Flow

```text
parse the command line
        ↓
validate role, level and format
        ↓
read the configuration when one is supplied
        ↓
resolve the storage target
        ↓
collect host facts
        ↓
decide every check
        ↓
assemble the checks in the fixed public order
        ↓
derive the overall result
        ↓
render text or JSON
        ↓
exit 0, 1 or 2
```

Probing is sequential. There is nothing to gain from concurrency here, and a single-threaded run is easier to reason about and to test.

## Probe Abstraction

`internal/probe` exposes a `Probe` interface with one method per fact: OS, architecture, CPU count, total memory, storage, default interface, interface details, time synchronization, file-descriptor limits and the `xrpld` version. The Linux implementation reads `/proc`, `/sys`, `statfs`, `RLIMIT_NOFILE` and four fixed commands. The tests supply their own implementation, so verdicts never depend on the machine running the suite.

## Evaluation vs Report Ordering

The configuration is read before the storage target is resolved, because the target can come from `[node_db] path`. That is an internal ordering decision: the public report always lists checks in the fixed order documented in the README, built as one slice of literals rather than from a map.

## Host Checks

Architecture, CPU count and memory are compared against the thresholds of the selected profile. The CPU value is the number of logical CPUs available to the process and is always described that way. Memory is `MemTotal`; free and available memory are deliberately ignored, because the question is capacity, not current pressure.

## Storage Resolution

The target is `--data-dir`, else the configured NodeDB path, else `/`. A relative `--data-dir` resolves against the working directory, a relative NodeDB path against the configuration's directory. If the target does not exist yet, the nearest existing ancestor is measured; nothing is ever created.

The filesystem is measured with `statfs`, and the verdict uses total capacity. The device comes from `/proc/self/mountinfo`: mount points are decoded (`\040`, `\011`, `\012`, `\134`), matched by path component so `/var/lib` never matches `/var/library`, and the longest match wins. The `major:minor` pair leads to `/sys/dev/block`, and the walk upwards from the resolved device finds `queue/rotational`. When it cannot be found, the media is unknown, which warns.

## Network Resolution

An explicit `--interface` is used as given; otherwise the IPv4 default route is read from `/proc/net/route`, requiring the `RTF_UP` flag and preferring the lowest metric. The name is validated through `net.InterfaceByName` before any sysfs path is built from it, and only then are `operstate` and `speed` read. Addresses, gateways and hardware addresses are never reported.

## Local Command Runner

Four executables may be run: `timedatectl`, `chronyc`, `systemctl` and `xrpld`. Their names and arguments are literals in the source; nothing from the configuration or the command line can select a program or an argument. Every call goes through `exec.CommandContext` with a timeout and `LC_ALL=C`, never through a shell.

## Bounded Command Output

Child output goes to a capped writer that stores at most 64 KiB while accepting every write, so a chatty child is neither able to exhaust memory nor to block on a writer that stopped draining. The writer is mutex-protected because stdout and stderr share one instance. Text that reaches a report is reduced to a single line with control characters removed.

## xrpld Config Extraction

The file must be a regular file within 1 MiB, valid UTF-8, with line endings normalized. Sections, keys and known identifiers are matched without case; values are kept as written. The parser retains only what the checks use: node size, NodeDB type and path, listener `ip`, `port` and `protocol`, whether `[port_grpc]` exists, and whether the two credential sections carry a value.

## Server Defaults and Effective Listeners

`[server]` is a mixed section: a line with `=` is a default, a bare line names an active listener. The effective settings of a listener are the server defaults overridden by its own section, which is how `xrpld` reads them. `[port_grpc]` is separate: it neither appears in `[server]` nor declares a protocol.

## Validator Checks

Credential checks see booleans only. The configuration mode check reads POSIX permission bits and claims nothing more. The bind review classifies the effective IP of every API listener and of the gRPC stanza as loopback, non-loopback or unclassifiable, and a non-loopback bind is a warning for operator review.

## Report Model

`internal/model` holds the statuses, the check, the summary counters and the report, with `schemaVersion` 1. Any failure makes the result `not_ready`, warnings alone make it `ready_with_warnings`, and skipped checks never downgrade anything.

## Determinism

There is no timestamp, duration or identifier in the report, no map iteration decides order or content, and the text layout uses fixed columns rather than terminal width. The same facts and inputs always render the same bytes.

## Security Boundary

Every operation is read-only. Configuration input is untrusted and bounded; secrets stay inside the parsing buffer; child output and paths are sanitized before they are rendered; no executable is selected by input; and no code path performs a network request.

## Non-Goals

No installation, no remediation, no monitoring, no benchmarking, no remote hosts, no daemon, no HTTP server, no metrics, no database, no container images, and no live XRPL state.
