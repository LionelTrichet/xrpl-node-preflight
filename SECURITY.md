# Security

## Threat Model

Assets: the integrity of the host being inspected, and the confidentiality of the validator credentials in the configuration it reads.

In scope: a configuration file that is hostile, malformed, oversized or full of control characters; a child process that never exits or floods its output; report output that is pasted into a terminal or a CI log; and any accidental disclosure of host identity or credentials.

Out of scope: a compromised host or operator, the correctness of the kernel, and anything about the network the host sits on.

## Read-Only Operation

The tool never modifies the host. It does not write or change `xrpld.cfg`, system configuration, file permissions, ownership, systemd units, ulimits or sysctl values; it installs nothing; it does not start, stop or restart `xrpld`; it does not touch firewall rules; and it generates no validator keys or tokens. Even resolving a storage target that does not exist yet only walks upward to an existing path — no directory is created.

## Untrusted Config Input

The supplied configuration is untrusted local input. It is parsed as data: there is no code execution, no shell interpolation, no include processing, no environment substitution and no lookup triggered by its content. No value in it can select an executable or an argument.

## Bounded Config Reading

The file must be a regular file of at most 1 MiB. It is opened once, checked through the open file, and read through a limit reader that asks for one byte beyond the maximum, so a file that grows between the check and the read is rejected rather than truncated. Invalid UTF-8 is refused.

## Secret Handling

The source configuration bytes necessarily contain any configured validator secrets while the file is being parsed. XRPL Node Preflight does not copy those secret values into its parsed fact model, report model, errors, or output.

`[validator_token]` and `[validation_seed]` are recorded as two booleans. No token or seed text, length, prefix or hash is retained anywhere, and the test suite fails if a fixture secret appears in the facts, in a check or in either renderer.

## Display Sanitization

Every externally derived string that reaches a report is sanitized: control characters are removed, output is reduced to a single line, and paths from the configuration or the command line are quoted. A configuration cannot inject a newline, a fake report line or a terminal escape sequence into the output.

## Command Execution

Four executable names may be run, and they are literals in the source: `timedatectl`, `chronyc`, `systemctl` and `xrpld`. Arguments are fixed in the same way. Nothing from the configuration or the command line selects what is executed.

## No Shell

Commands are executed through `exec.CommandContext` with an explicit argument vector. There is no `sh -c`, no `bash -c` and no string interpolation into a command line.

## Command Timeouts

Every child process runs under a context timeout: two seconds for the systemd and chrony probes, three for the `xrpld` version. A probe that cannot answer produces a warning rather than a hang.

## Bounded Child Output

Child output is stored in a capped writer that keeps at most 64 KiB and still accepts every write, so a child cannot exhaust memory and cannot block on a reader that stopped consuming. Only a single sanitized line of it is ever displayed.

## No External Network Access

The `check` command performs no external network request: no HTTP, HTTPS, XRPL RPC, WebSocket, external DNS lookup, release lookup, package index or port scan. The local probes inspect local daemon state only and never query a public time server. Dependency downloads and `govulncheck` in CI are development concerns, not runtime behaviour.

## Validator Exposure Limitations

The bind review reads the local listener configuration. A non-loopback bind is a prompt for review, not evidence of reachability: firewalls, NAT, reverse proxies, cloud security groups and routing are outside the tool's view. No port is bound, connected to or scanned.

## Config Mode Limitations

The configuration mode check evaluates POSIX permission bits only. A PASS means the file is owner-readable with no group or other bits. It says nothing about the file owner, the `xrpld` service user, ACLs, SELinux, AppArmor, filesystem encryption or the mode of the parent directory.

## Host Identity Minimization

Reports never carry a hostname, machine ID, user name, home directory, IP address, hardware address, cloud instance identifier or public address. Paths appear only when the operator supplied them or the configuration named them, and they are quoted.

## Known Limitations

- No complete `xrpld` configuration validation.
- No verification of file ownership, ACLs or mandatory access control.
- No firewall, NAT or reachability verification.
- No storage benchmarking, so media detection is a kernel report rather than a performance guarantee.
- No live XRPL state: synchronization, consensus participation and validator agreement are not observed.
- Duplicate keys inside one configuration section are resolved by the last value seen, as the upstream format allows.

## Vulnerability Reporting

Please report vulnerabilities privately through GitHub's **Report a vulnerability** option on this repository rather than in a public issue, with the affected component and steps to reproduce.
