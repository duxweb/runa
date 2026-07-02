# Changelog

English | [简体中文](CHANGELOG.zh-CN.md)

## v0.1.2 - 2026-07-02

### Changed

- Optimized the Redis queue driver storage format from whole-job JSON strings to Redis hashes, so state transitions update only mutable fields.
- Reduced Redis queue hot-path round trips: Reserve now claims and updates jobs in Lua, Ack and Fail run as single Lua scripts, Release updates fields without reading the full body, and List/Purge use batched hash reads.
- Added opt-in real Redis benchmarks for queue push, reserve/ack, and worker throughput via `RUNA_REDIS_BENCH_ADDR`.

### Fixed

- Preserved corrupt-job self-healing for Redis queues without poisoning an entire reserved batch.
- Kept `queue.Driver`, `queue.JobMessage`, and worker APIs unchanged while moving Redis job bodies to hash storage.

## v0.1.1 - 2026-07-01

### Changed

- Unified `route.Context` generic accessors as Go 1.27 generic methods, including `Param`, `Query`, `Header`, `Cookie`, `Form`, `Meta`, `Input`, and `Service`.
- Changed route metadata reads to `route.MetaAs[T](...)` as a method on `*route.Route`.
- Split optional XLSX CRUD import/export support into `crud/excelize`, keeping Excel dependencies out of the base `crud` module.
- Renamed the message broker interface to `message.Driver` and updated official message drivers accordingly.
- Centralized terminal color detection through `core.ColorEnabled`.
- Shared view template helper utilities between `view` and `view/rhtml`.

### Fixed

- Removed the stale `internal/registry` copy in favor of `kernel/registry`.
- Removed redundant helper variants and duplicate `contains` implementations.
- Updated route, CRUD, audit, middleware, resource, console, and generated-code examples for the new generic method style.
- Updated English and Chinese documentation for route context helpers, CRUD Excelize, module installation, and injected route services.

## v0.1.0 - 2026-07-01

### Added

- Added the Runa micro-kernel: application startup, Provider lifecycle, Module lifecycle, DI, config, commands, host units, application time, and shutdown handling.
- Added HTTP route capability with typed route registration, request context, binding helpers, validation integration, response helpers, error rendering, route groups, middleware, and host startup.
- Added official middleware modules, including recover, request id, real ip, logger, CORS, CSRF, body limit, timeout, helmet, healthcheck, static, language negotiation, session, auth, rate, and audit integration.
- Added business capabilities for cache, queue, database, storage, session, auth, view, language/i18n, event, task, schedule, message, lock, rate, asset, RBAC, sanitize, and security presets.
- Added official drivers and adapters such as Redis, AMQP, MQTT, NATS, S3, Oro database, Oro CRUD store, Snowflake ID, WebSocket Redis, cluster Redis, and Prometheus observe integration.
- Added OpenAPI, resource routing, CRUD helpers, RHTML template integration, WebSocket transport, JSON-RPC transport, console, audit, observe, devtools, and examples.
- Added `runa` CLI tooling for development reload, code generation, introspection, doctor checks, MCP, and `llms.txt` generation.
- Added documentation site source under `docs/`, GitHub Pages workflow, CI workflow, release script, MIT license, security policy, contribution guide, and issue/PR templates.

### Known limitations

- This is a pre-1.0 release candidate; public APIs may still change before `v1.0`.
- The project currently requires Go `1.27rc1`.
- gRPC transport is planned but not implemented in this release.
- The first public release uses multi-module tags so each submodule can be consumed independently.
