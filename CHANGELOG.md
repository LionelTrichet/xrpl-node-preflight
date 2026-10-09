# Changelog

## [Unreleased]

## [0.1.0]

### Added

- Linux host readiness checks for xrpld.
- Production and minimum readiness profiles.
- Node and validator roles.
- Logical CPU and memory checks.
- Storage capacity and non-rotational media detection.
- Network interface and link-speed checks.
- Local time-synchronization checks.
- xrpld service file-descriptor checks with safe fallback.
- xrpld binary/version detection.
- Bounded secret-aware xrpld.cfg extraction.
- [server] listener-default inheritance.
- node_size and NodeDB checks.
- Peer-listener validation.
- Validator credential checks.
- Legacy validation-seed warnings.
- Validator configuration-mode checks.
- Validator HTTP/WebSocket/gRPC bind review.
- Deterministic text and JSON reports.
- Network-free runtime.

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
