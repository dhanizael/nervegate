# Contributing to NerveGate

Thank you for your interest in contributing to **NerveGate**! We welcome contributions from the community to help build nature's nervous system for AI agent infrastructure.

---

## Development Setup

1. **Prerequisites:**
   - Go 1.26+ installed on Linux
   - `make` automation tool

2. **Clone & Build:**
   ```bash
   git clone https://github.com/hxmdxnx/nervegate.git
   cd nervegate
   make build
   ```

3. **Running Quality Checks:**
   ```bash
   make test-race   # Run race detector unit tests
   make bench       # Run performance benchmarks
   make lint        # Run linter
   ```

---

## Pull Request Guidelines

- All PRs must pass `make test-race` with **zero data races**.
- Keep gateway overhead under sub-millisecond limits (< 50µs).
- Maintain 100% test coverage for new routing logic.
