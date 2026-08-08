# Changelog

All notable changes to **NerveGate** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0-alpha] - 2026-08-08

### Added
- **Core Architecture:** Dual Unix Socket (`/tmp/nervegate.sock`) and HTTP/2 (`:8080`) ingress engine.
- **Task Complexity Scorer:** Multi-pass heuristic classifier (Fast, Standard, Reasoning model tiers).
- **Key Pool Rotator:** Sliding-window rate limit state machine with HTTP 429 auto-failover.
- **Payload Trimmer:** RTK-style token compression reducing tool execution outputs by 20–40%.
- **Bifrost Engine:** Integrated multi-provider engine supporting Anthropic, OpenAI, Bedrock, Vertex AI, xAI, DeepSeek, and vLLM.
- **World-Class CLI Scaffolding:** Cobra CLI subcommands (`serve`, `benchmark`, `version`), Makefile automation, Docker containerization, and Systemd deployment manifests.
