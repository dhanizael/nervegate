# NerveGate

> **The Sub-Millisecond Intelligent Gateway & Model Orchestrator for Linux**

`NerveGate` is a high-performance, Linux-native LLM proxy gateway designed to route requests with microsecond-level latency overhead (< 50µs). It acts as nature's nervous system for AI agents, dynamically evaluating work type, task complexity, and criticality to dispatch requests to the optimal model and API key tier.

---

## Key Features

- **Microsecond Latency:** Built in Go with zero-copy HTTP/2 and Unix Domain Socket (`/tmp/nervegate.sock`) support.
- **Task Complexity Classifier:** Multi-pass heuristic scorer (0–100) mapping work types to Fast, Standard, or Reasoning model tiers.
- **Resilient Key Rotation:** Sliding-window rate limit tracking (RPM/TPM) and zero-latency failovers across multi-account key pools.
- **Payload Token Compression (RTK):** Intelligent trimming of verbose tool outputs (`git diff`, stack traces, build logs) saving 20–40% on token consumption.
- **Universal Provider Support:** Native OpenAI, Anthropic, Gemini, DeepSeek, and OpenAI-compatible endpoints.

---

## Quick Start

```bash
# Build NerveGate
go build -o nervegate ./cmd/nervegate

# Start the daemon
./nervegate start --port 8080 --socket /tmp/nervegate.sock
```

---

## License

MIT
