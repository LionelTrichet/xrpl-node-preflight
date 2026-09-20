# XRPL Node Preflight

**Go CLI for Linux host and xrpld config preflight checks before running XRP Ledger nodes and validators.**

XRPL Node Preflight is a Go CLI that checks whether a Linux host and optional `xrpld` configuration meet the operational prerequisites for running an XRP Ledger node or validator.

It checks host capacity, storage type, network link, time synchronization, file-descriptor limits, selected `xrpld` settings and validator-specific security conditions without deploying `xrpld`, exposing secrets or making external network requests.

## Why

Node deployment problems are easier to identify before `xrpld` starts than after a server begins falling behind the network. XRPL Node Preflight turns selected host and configuration prerequisites into deterministic local checks that can be reviewed by an operator or consumed in CI.

```text
Linux host + xrpld.cfg
          ↓
   local preflight
          ↓
  PASS / WARN / FAIL
          ↓
    operator / CI
```

## Architecture

```text
Linux Host
   │
   ├── OS, architecture, logical CPUs, memory
   ├── storage target, capacity, media
   ├── network interface and link speed
   ├── time synchronization
   └── file-descriptor limits
            │
xrpld.cfg ──┤
            │
            ▼
    XRPL Node Preflight
            │
      ┌─────┼──────┬──────┐
      ▼     ▼      ▼      ▼
    PASS   WARN   FAIL   SKIP
            │
            ├── text
            └── JSON
```

Everything is read-only: **inspect locally, report deterministically, change nothing.**

## What It Checks

Host: operating system, architecture, logical CPUs, total memory.
Storage: which filesystem holds the xrpld data, its total capacity, and whether the kernel reports non-rotational media.
Network: the selected interface, its link state and its link speed.
Operations: local time synchronization, the file-descriptor limit of `xrpld.service`, and whether an `xrpld` binary is installed.
Configuration: `node_size`, the NodeDB stanza and the peer listener.
Validator: credentials, legacy `validation_seed`, configuration file mode, and where API and gRPC listeners bind.

## What This Is / Is Not

This is a Linux readiness CLI, an XRPL operational preflight, an `xrpld` configuration inspector, a validator pre-deployment checker and a CI-friendly report generator.

This is not an installer, a monitoring agent, a validator manager, a firewall scanner, a benchmark suite, a remote-management tool, a security certification tool or a full `xrpld` configuration validator.

**XRPL Node Preflight performs no external network requests. Runtime checks use local Linux state, the supplied `xrpld.cfg`, and fixed local command-line probes only.**

## Quick Start

```bash
go build -trimpath -o bin/xrpl-node-preflight ./cmd/xrpl-node-preflight

./bin/xrpl-node-preflight check
./bin/xrpl-node-preflight check --config /etc/xrpld/xrpld.cfg
./bin/xrpl-node-preflight check --role validator --config /etc/xrpld/xrpld.cfg
./bin/xrpl-node-preflight check --config /etc/xrpld/xrpld.cfg --format json
./bin/xrpl-node-preflight check --data-dir /var/lib/xrpld/db --interface eth0
```

```text
usage: xrpl-node-preflight check [--role node|validator] [--level production|minimum] [--config PATH] [--data-dir PATH] [--interface NAME] [--format text|json]
```

Exit codes: `0` when the report contains no failure, `1` when at least one check failed, `2` for a usage or tool error. A warning never changes the exit code.

## Roles and Levels

`--role node` (default) runs the host and configuration checks. `--role validator` adds the five validator checks.

`--level production` (default) applies the recommended profile: amd64, at least 8 logical CPUs, at least 64 GiB of memory, at least 50 GB of total filesystem capacity, non-rotational media, a gigabit link, synchronized time and an `xrpld.service` file-descriptor limit of at least 65536.

`--level minimum` applies the lower profile: at least 4 logical CPUs and 16 GiB of memory, with the same storage requirements; arm64, a slower link, unsynchronized time and a low verified service limit become warnings instead of failures. This profile is not a Mainnet production certification: XRPL's lower specifications are not sufficient for reliably staying synchronized with Mainnet.

## Checks

Checks always appear in this order, and the identifiers are part of the machine-readable contract:

| ID | What it reports |
|----|-----------------|
| `host.os` | the operating system is Linux |
| `host.arch` | the CPU architecture |
| `host.cpu` | logical CPUs available to the process |
| `host.memory` | `MemTotal` from `/proc/meminfo` |
| `storage.target` | which path was measured and why |
| `storage.capacity` | total and available capacity of that filesystem |
| `storage.media` | whether the kernel reports non-rotational media |
| `network.interface` | link state and speed of the selected interface |
| `time.sync` | local synchronization state |
| `limits.nofile` | `xrpld.service` LimitNOFILE, or the process limit as an indication |
| `xrpld.binary` | whether `xrpld` is installed and what version it reports |
| `config.file` | whether the supplied configuration could be read |
| `config.node_size` | the configured node size |
| `config.node_db` | NodeDB backend and path |
| `config.peer_port` | the single peer listener and its port |
| `validator.config` | whether a validator configuration was supplied |
| `validator.credentials` | validator token or legacy seed |
| `validator.legacy_seed` | presence of `[validation_seed]` |
| `validator.config_mode` | POSIX mode bits of the configuration file |
| `validator.api_bind` | where API and gRPC listeners bind |

The storage target is chosen in a fixed order: `--data-dir`, then the `[node_db]` path of a parsed configuration, then the root filesystem. A relative `--data-dir` is resolved against the working directory; a relative NodeDB path is resolved against the directory of the configuration file, as `xrpld` does. When the target does not exist yet, the nearest existing ancestor is measured, and nothing is created.

**CPU readiness uses logical CPUs available to the process as a practical approximation of the XRPL core-count recommendation. The tool does not determine physical-core topology or sustained processor frequency.**

**A non-rotational storage PASS means only that the Linux kernel reports non-rotational media. The tool does not benchmark sustained IOPS, certify storage latency, or determine whether every cloud block-storage product is appropriate for xrpld.**

**The 50 GB check uses total filesystem capacity. Available space is reported separately, and operators remain responsible for provisioning sufficient headroom for their selected ledger-history retention policy.**

**A systemd `xrpld.service` LimitNOFILE value can be evaluated directly. When that service-specific limit is unavailable, the current preflight process limit is shown only as an indication and produces a warning rather than a service-level failure.**

## Text Output

```text
XRPL Node Preflight

Role:  validator
Level: production

PASS  host.os                     Linux
PASS  host.arch                   amd64
PASS  host.cpu                    16 logical CPUs
PASS  host.memory                 64.0 GiB total memory
PASS  storage.media               kernel reports non-rotational media
PASS  network.interface           eth0, 1000 Mbit/s
WARN  xrpld.binary                xrpld not found in PATH
PASS  validator.credentials       validator token present
PASS  validator.config_mode       owner-readable; no group/other mode bits
WARN  validator.api_bind          validator API binds to a non-loopback interface; verify firewall and network exposure

Result: READY WITH WARNINGS
10 checks: 8 pass, 2 warn, 0 fail, 0 skip
```

There are no colors, symbols, spinners or terminal-width tricks: the output is the same in a CI log, over SSH and in a redirected file. The three results are `READY`, `READY WITH WARNINGS` and `NOT READY`.

## JSON Output

```json
{
  "schemaVersion": 1,
  "role": "validator",
  "level": "production",
  "result": "ready_with_warnings",
  "checks": [
    {
      "id": "host.memory",
      "status": "pass",
      "summary": "64.0 GiB total memory",
      "observed": "64.0 GiB",
      "expected": ">= 64 GiB"
    }
  ],
  "summary": { "pass": 8, "warn": 2, "fail": 0, "skip": 0 }
}
```

The report carries no timestamp, duration, run identifier or host identity, so equivalent facts always produce byte-identical output.

## xrpld Configuration Checks

`--config` is optional. Without it the four configuration checks are skipped and the host checks still run.

The file must be a regular file of at most 1 MiB and valid UTF-8; DOS, UNIX and classic Mac line endings are all accepted. Section names, keys and known identifiers are compared without case, while values are kept as written. When the configuration cannot be read, that is one failure on `config.file` and the checks depending on it are skipped rather than turned into further failures.

**The configuration reader extracts only the fields needed by implemented checks and applies `[server]` defaults when evaluating active listeners. It is not a complete parser or validator for every xrpld configuration option.**

`node_size` may be omitted, in which case `xrpld` selects one. For the production profile `huge` passes and everything else warns; `large` in particular is discouraged by current capacity guidance. NodeDB must declare `type` and `path`: NuDB passes and RocksDB warns as a legacy backend.

**XRPL Node Preflight does not require a fixed peer port. Port 2459 is the current IANA-assigned XRP Ledger peer port, while older deployments may use 51235 and operators may configure another valid port.** Exactly one active listener may serve the peer protocol; none warns and more than one fails.

## Validator Checks

**A validator token is the recommended modern validator credential. A legacy `validation_seed` is recognized as capable of configuring validation but produces a warning because the validator-token workflow is preferred.**

**Validator token and validation-seed values are never copied into reports or the parsed configuration model. The tool records only whether those credential sections contain a value.**

The configuration mode check passes when the file is owner-readable with no group or other permission bits, for example `0600` or `0400`. It does not inspect the file owner, the `xrpld` service user, ACLs, SELinux, AppArmor, filesystem encryption or the mode of the parent directory. A PASS means only: owner-readable and no group/other POSIX permission bits.

**A non-loopback validator API or gRPC listener produces a warning, not proof of public Internet exposure. Firewalls, NAT, reverse proxies and cloud security groups are outside this tool's scope.**

**XRPL recommends that validators not be publicly accessible. XRPL Node Preflight reviews local listener configuration only; it does not verify firewalls, NAT, reverse proxies, cloud security groups or actual external reachability.** `[port_grpc]` is reviewed even though it stands outside `[server]`; when its `ip` is omitted, the documented loopback default is assumed rather than a public bind.

## Security Model

The supplied configuration is untrusted local input: it is read with a size bound, validated as UTF-8, and parsed without shell interpolation, include processing, environment substitution or any external lookup. No value in it ever selects an executable.

The four programs this tool may run are fixed in source — `timedatectl`, `chronyc`, `systemctl` and `xrpld` — always without a shell, always with a timeout, and with their output bounded to 64 KiB and stripped of control characters before it reaches a report. Paths from the configuration or the command line are quoted, so nothing can inject a line or an escape sequence into the output.

Nothing is modified: no configuration, permission, unit, limit, firewall rule or package. No validator key or token is generated, read or reported. See [SECURITY.md](SECURITY.md).

## Operational Limitations

- no physical-core topology validation
- no CPU-frequency validation
- no disk IOPS benchmark
- no storage-latency benchmark
- no AWS EBS detection or certification
- no external bandwidth test
- no firewall verification
- no cloud-security-group verification
- no actual external reachability test
- no live `xrpld` synchronization check
- no consensus participation check
- no validator agreement check
- no UNL check
- no latest `xrpld` release lookup
- no file ownership, ACL or SELinux verification

**XRPL recommends high-performance SSD/NVMe storage with sustained IOPS and specifically warns against AWS EBS for reliable synchronization. XRPL Node Preflight does not benchmark IOPS, latency, or identify every cloud block-storage product, so a storage-media PASS is not a complete production-storage certification.**

**Passing the 50 GB check does not certify enough free-space headroom for a chosen ledger-history retention policy. Disk requirements can grow substantially with retained ledger history.**

## What It Does Not Check

The tool never contacts the XRP Ledger, a peer, an RPC server, a package repository or a release API, and never looks up the newest `xrpld` version. It does not install, start, stop or restart anything, does not open ports and does not repair the host. Version output is reported as text, never classified as supported or outdated.

## Development

```bash
make check   # fmt-check, vet, test, test-race, vuln, build
```

Go 1.27.1, no runtime dependencies, standard library only.

## Testing

```bash
go test ./...
go test -race ./...
```

Host-dependent behaviour is covered through a fake probe and configuration fixtures, so the suite needs no `xrpld`, no systemd unit, no validator credentials, no XRPL network, no Docker and no cloud provider.

## Upstream References

Behaviour follows the official XRPL documentation: System Requirements, Capacity Planning, "xrpld Server Won't Start", "Run xrpld as a Validator", the Peer Protocol, "Configure gRPC", the `xrpld` command line usage, and the example configuration in `XRPLF/rippled`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache-2.0. See [LICENSE](LICENSE).
