# NerveGate

> **Low-latency AI gateway and model orchestrator for Linux.**

`NerveGate` is a Go LLM proxy daemon. It listens on a Unix domain socket
(`/tmp/nervegate.sock`) and TCP (`:8080`) and applies three operations to
inbound chat-completion requests before forwarding them upstream:

1. **Payload Compression:** byte-slice transformation that collapses runs of
   whitespace and duplicate newlines in tool outputs (pooled buffers, single
   allocation per request).
2. **Complexity Classification:** heuristic multi-pass scoring that maps a
   request to a model tier (`FAST`, `STANDARD`, `REASONING`) and flags
   criticality. Tiers are currently exposed as response metadata headers and
   are the hook for future model routing.
3. **Multi-Key Pool Failover:** round-robin key selection with a sliding-window
   rate limiter (RPM/TPM per key) and automatic cool-down when upstream
   responds `429` or `5xx`.

```
[Agent Process] ---> (:8080 | /tmp/nervegate.sock) ---> [Trimmer] ---> [Classifier] ---> [Rotator] ---> [Upstream]
```

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

The daemon proxies to `https://api.openai.com` by default. Configure an
upstream and a key pool with environment variables:

```bash
export NERVEGATE_UPSTREAM=https://api.openai.com   # any OpenAI-compatible base URL
export NERVEGATE_PROVIDER=openai                   # provider pool name
export NERVEGATE_KEYS_OPENAI=sk-aaa,sk-bbb         # comma-separated key pool
export NERVEGATE_RPM_OPENAI=60                     # per-key requests/minute (0 = unlimited)
export NERVEGATE_TPM_OPENAI=0                      # per-key tokens/minute (0 = unlimited)
./bin/nervegate serve
```

When a key pool is configured, the gateway injects the rotated key and the
caller's `Authorization` header is ignored. Without a pool, the caller's own
`Authorization` header is passed through unchanged.

### OpenAI API Proxy Example

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "user", "content": "Fix deadlock in mutex handler"}
    ]
  }'
```

Response metadata is returned in the headers:

| Header | Meaning |
| :--- | :--- |
| `X-NerveGate-Latency-Us` | Total request latency (gateway + upstream) in microseconds |
| `X-NerveGate-Tier` | Classified model tier (`FAST`/`STANDARD`/`REASONING`) |
| `X-NerveGate-Score` | Complexity score (0–100) |
| `X-NerveGate-Trimmed-Bytes` | Bytes removed by the payload trimmer |

## Security Notes

- The TCP listener binds to all interfaces and is unauthenticated — front it
  with a TLS-terminating reverse proxy and an allowlist in production.
- The Unix socket is created with mode `0600`; clients must run as the same
  user as the daemon.
- Request bodies are capped at 32 MiB by default.

## Performance

`make bench` runs the microsecond benchmarks:

```text
BenchmarkTrimmer_TrimBytes-20    7606959    156.3 ns/op    64 B/op    1 allocs/op
```

## Development

```bash
make test-race   # Unit tests with the Go race detector
make bench       # Microsecond latency benchmarks
make lint        # golangci-lint (falls back to go vet)
```

## License

MIT — see [LICENSE](LICENSE). `pkg/bifrost_core/` is a vendored snapshot of
the Apache-2.0 licensed [Maxim Bifrost](https://github.com/maximhq/bifrost)
core (see [NOTICE](NOTICE)).
