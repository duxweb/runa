# Changelog

English | [简体中文](CHANGELOG.zh-CN.md)

## v0.1.0 - Unreleased

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
- Final publishing requires the repository tags to be created after release validation.
