# Changelog

All notable changes to **NerveGate** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **Build was broken from a fresh clone:** the `.gitignore` rule `nervegate`
  matched any path with that name, silently excluding `cmd/nervegate/commands/`
  from git. Rules are now scoped to `/nervegate` and `/bin/`.
- **Non-ASCII response header dropped on the wire:** `X-NerveGate-Latency-µs`
  was silently stripped by net/http (header names must be ASCII). Renamed to
  `X-NerveGate-Latency-Us` and verified end-to-end.
- **Latency metric measured before proxying:** the gateway now measures the
  full round trip through the upstream.
- **Module path mismatch:** `go.mod` and imports now use
  `github.com/dhanizael/nervegate` instead of a stale personal path.
- **Trimmer "zero-allocation" claim:** the function always allocates the
  result slice; docs updated, and pooled buffers larger than 64 KiB are now
  discarded instead of being retained forever.

### Added
- **Real key rotation in the request path:** the rotator now implements a
  sliding-window RPM/TPM rate limiter per key, and the ingress handler selects
  a key, injects it as `Authorization`, marks cool-downs on `429`/`5xx`, and
  records token usage from upstream responses.
- **Configuration via environment:** `NERVEGATE_UPSTREAM`,
  `NERVEGATE_PROVIDER`, `NERVEGATE_KEYS_<PROVIDER>`, `NERVEGATE_RPM_<PROVIDER>`,
  `NERVEGATE_TPM_<PROVIDER>`.
- **Request hardening:** 32 MiB body cap (HTTP 413 on overflow), strict JSON
  validation (HTTP 400), Unix socket mode `0600`, graceful listener shutdown
  that removes the socket file.
- **Licensing:** root MIT `LICENSE`, `NOTICE` pinning the vendored Maxim
  Bifrost core to upstream commit `2c6f1328f6c4`, and the verbatim Apache-2.0
  text at `pkg/bifrost_core/LICENSE`.

### Security
- The TCP listener is unauthenticated and binds all interfaces; document
  fronting it with a TLS-terminating reverse proxy in production.

## [v0.1.0-alpha] - 2026-08-08

### Added
- **Core Architecture:** Dual Unix Socket (`/tmp/nervegate.sock`) and HTTP/2 (`:8080`) ingress engine.
- **Task Complexity Scorer:** Multi-pass heuristic classifier (Fast, Standard, Reasoning model tiers).
- **Key Pool Rotator:** Sliding-window rate limit state machine with HTTP 429 auto-failover.
- **Payload Trimmer:** RTK-style token compression reducing tool execution outputs by 20–40%.
- **Bifrost Engine:** Integrated multi-provider engine supporting Anthropic, OpenAI, Bedrock, Vertex AI, xAI, DeepSeek, and vLLM.
- **World-Class CLI Scaffolding:** Cobra CLI subcommands (`serve`, `benchmark`, `version`), Makefile automation, Docker containerization, and Systemd deployment manifests.
