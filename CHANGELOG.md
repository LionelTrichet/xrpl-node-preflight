# Changelog

## [Unreleased]

### Fixed

- A command that leaves a background process holding its output no longer
  keeps the check waiting past its timeout.

### Security

- Built with Go 1.27.3. Go 1.27.1 carries standard library vulnerabilities
  that are fixed in 1.27.2 and 1.27.3; govulncheck found none of them
  reachable from this code.

## [0.1.0]

### Added

- Linux host readiness checks for `xrpld`
- production and minimum readiness profiles
- node and validator roles
- logical CPU and total memory checks
- storage capacity and non-rotational media detection
- network interface and link-speed checks
- local time-synchronization checks
- `xrpld.service` file-descriptor checks with a safe fallback
- local `xrpld` binary and version detection
- bounded, secret-aware `xrpld.cfg` extraction
- `[server]` listener-default inheritance
- `node_size` and NodeDB checks
- peer-listener validation
- validator credential checks
- legacy `validation_seed` warnings
- validator configuration-mode checks
- validator HTTP, WebSocket and gRPC bind review
- deterministic text and JSON reports
- network-free runtime operation
