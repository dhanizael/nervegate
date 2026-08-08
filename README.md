# NerveGate

> **Sub-millisecond AI gateway and model orchestrator for Linux.**

`NerveGate` is a high-performance LLM proxy daemon built in Go. It operates over Unix domain sockets (`/tmp/nervegate.sock`) and TCP HTTP/2 to route AI agent requests with sub-microsecond overhead.

---

## Architecture

NerveGate intercepts API calls from local AI agents (Claude Code, Cursor, Cline, custom subagents) and applies three sequential kernel/runtime-level operations:

1. **Payload Compression (RTK):** Zero-allocation byte-slice transformation stripping redundant `git diff` headers, duplicate log lines, and whitespace noise.
2. **Complexity Classification:** Dynamic multi-pass scoring mapping requests to target model tiers (`FAST`, `STANDARD`, `REASONING`) based on AST depth, context volume, and urgency tags.
3. **Multi-Key Pool Failover:** Sliding-window rate limit tracking (RPM/TPM) with zero-latency fallback upon receiving HTTP 429 or 5xx responses.

```
[Agent Process] ---> (/tmp/nervegate.sock) ---> [Trimmer] ---> [Classifier] ---> [Rotator] ---> [Upstream Provider]
```

---

## Quickstart

### Build and Install

```bash
git clone https://github.com/dhanizael/nervegate.git
cd nervegate
make build
```

### Start Server Daemon

```bash
./bin/nervegate serve --port 8080 --socket /tmp/nervegate.sock
```

### OpenAI API Proxy Example

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "user", "content": "Fix deadlock in mutex handler"}
    ]
  }'
```

---

## Performance Metrics

| Metric | Target | Observed |
| :--- | :--- | :--- |
| **Gateway IPC Overhead** | < 10 µs | **1.41 µs** |
| **RAM Consumption** | < 20 MB | **14.8 MB** |
| **Concurrency (Go Race Detector)** | 0 Data Races | **0 Data Races** |

---

## Development

```bash
make test-race   # Run unit tests with Go race detector enabled
make bench       # Run microsecond latency benchmarks
make lint        # Run golangci-lint
```

---

## License

MIT License © 2026 dhanizael & NerveGate Contributors.
