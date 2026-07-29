# LuaSpider

LuaSpider 是一个 Go 宿主驱动的 Lua 规则抓取引擎。Go 负责 HTTP API、
并发调度、受控网络访问和 MySQL 持久化，Lua 规则负责页面请求、字段提取
和链接发现，并支持文件热重载。

当前仓库处于 Redis Streams 改造前的单进程基线阶段。README 只描述已经
实现并验证的能力；分布式消费、故障接管和重试仍属于下一阶段。

## Current architecture

```text
POST /api/v1/task
        |
        v
bounded channel -- full --> HTTP 429
        |
        v
fixed worker pool
        |
        v
isolated Lua VM -- Context timeout
        |
        +--> restricted HTTP + goquery helpers
        |
        v
MySQL upsert
```

## Implemented

- Gin task submission and result query APIs.
- Bounded in-process channel and configurable fixed worker pool.
- One isolated gopher-lua `LState` per task.
- VM-level cancellation through `context.WithTimeout` and `L.SetContext`.
- Restricted Lua standard libraries and context-aware HTTP requests.
- HTTP/HTTPS validation, private literal IP rejection, redirect revalidation and
  a 4 MiB response limit.
- Lua rule hot reload with the last successfully read in-memory script retained
  on file read failure.
- MySQL persistence with a configured connection pool and business unique index.
- Optional localhost-only pprof endpoint.
- Unit tests and a resource-limited Docker benchmark environment.

## Known boundaries

- Accepted tasks exist only in the process channel. A crash loses queued tasks;
  the measured baseline is documented in
  [`CHANNEL-CRASH-001`](docs/benchmarks/CHANNEL-CRASH-001.md).
- The current result model and default rule are GitHub-specific.
- There is no durable task state, retry, dead-letter queue or multi-instance
  coordination yet.
- Lua rules are trusted repository files. Context limits execution time but is
  not a per-script hard memory sandbox.
- MySQL is a single instance in the current development environment.

## Quick start

1. Start MySQL and create the database named by `mysql.dsn`.
2. Update `configs/config.yaml` for the local MySQL and optional proxy settings.
3. Start the complete server package:

```powershell
go run ./cmd/server
```

Submit a task:

```bash
curl -X POST http://localhost:8080/api/v1/task \
  -H "Content-Type: application/json" \
  -d '{"target":"vue","url":"https://github.com/vuejs/vue"}'
```

Query the current result:

```bash
curl "http://localhost:8080/api/v1/task?repo=vuejs/vue"
```

## Lua host API

The Go host currently exposes these functions to trusted Lua rules:

- `http_get(url)`: fetch an HTTP/HTTPS response with task cancellation.
- `html_find(html, selector)`: return the first matching element's text.
- `html_find_all(html, selector)`: return all matching text values.
- `html_attr_all(html, selector, attribute)`: return an attribute from every
  matching element.
- `url_resolve(base, reference)`: resolve a relative link against its base URL.

A rule returns `(true, result_table)` on success or `(false, error_message)` on
failure. The current result table contains `url`, `stars`, `description` and
optional `fission_urls`.

## Verification

```powershell
go test ./...
go vet ./...
go build ./...
```

Preserved evidence:

- [`BASELINE-CHANNEL-001`](docs/benchmarks/BASELINE-CHANNEL-001.md): worker 1/2/4
  throughput under fixed resources.
- [`SATURATION-WORKER-4-001`](docs/benchmarks/SATURATION-WORKER-4-001.md):
  four-worker saturation follow-up.
- [`LUA-TIMEOUT-001`](docs/verification/LUA-TIMEOUT-001.md): repeated Lua
  deadline interruption.
- [`CHANNEL-CRASH-001`](docs/benchmarks/CHANNEL-CRASH-001.md): in-process queue
  crash-loss boundary.

## Next release gate

The next release replaces the process channel with a MySQL task state machine
and Redis Streams Consumer Group. It will add idempotent submission, ACK after
database commit, multi-worker consumption and recoverable pending messages.
