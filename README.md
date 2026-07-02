<p align="center">
  <h1 align="center">Runa</h1>
</p>

<p align="center">
  <b>A Go web framework for business applications.</b><br/>
  Start with a lean micro-kernel and install capabilities on demand, from a single route to an enterprise full-stack application.
</p>

<p align="center">
  <a href="https://duxweb.github.io/runa/">Docs</a> &middot;
  <a href="https://duxweb.github.io/runa/zh-cn/">中文文档</a> &middot;
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="https://github.com/duxweb/runa/issues">Feedback</a>
</p>

<p align="center">
  English | <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/duxweb/runa/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/duxweb/runa/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/duxweb/runa/actions/workflows/docs.yml"><img alt="Docs" src="https://github.com/duxweb/runa/actions/workflows/docs.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/duxweb/runa"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/duxweb/runa.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-black.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27rc1+-00ADD8.svg">
</p>

---

## Why Runa

| Common framework friction | Runa's answer |
| :--- | :--- |
| Small apps want only HTTP, large apps need much more | Start with `runa` + `route`, then add cache, queue, database, auth, storage, views, i18n, observability, and drivers when needed. |
| Full-stack frameworks often pull dependencies you never use | Each capability is a separate Go module. If you do not import it, it does not enter your compile path. |
| Router-first design makes background workers and CLI feel secondary | The kernel owns lifecycle, DI, config, commands, modules, and host units. HTTP is a pluggable transport. |
| Business projects need structure, not just handlers | Modules, typed routes, explicit validation, resources, CRUD helpers, OpenAPI, queue, schedule, and view integration target business development directly. |

## Status

Runa is a **pre-1.0 public preview**. The current public release target is `v0.1.2`, so APIs may still change before `v1.0`.

Runa currently targets **Go 1.27rc1**. Use a Go 1.27 release candidate toolchain until the stable Go 1.27 toolchain is available.

## Install

Install only the modules you need:

```bash
go get github.com/duxweb/runa
go get github.com/duxweb/runa/route
```

## Quick Start

```go
package main

import (
    "context"

    "github.com/duxweb/runa"
    "github.com/duxweb/runa/route"
)

func main() {
    app := runa.New()
    app.Install(route.Provider(route.Addr(":8080")))

    route.Default().Get("/", func(ctx *route.Context) error {
        return ctx.JSON(runa.Map{"message": "Hello Runa"})
    })

    if err := app.Run(context.Background()); err != nil {
        panic(err)
    }
}
```

Run it:

```bash
go run .
curl http://localhost:8080/
```

## Architecture

Runa is split into four layers:

```text
Kernel        github.com/duxweb/runa
Transports    route / ws / jsonrpc / future grpc
Capabilities  cache / queue / database / storage / auth / session / view / lang ...
Drivers       redis / s3 / amqp / nats / oro ...
```

The root module provides application startup, dependency injection, config, commands, modules, lifecycle, hosts, errors, and application time. It does not bind HTTP, cache, database, or queue by default.

## Capabilities

| Capability | Install path | Purpose |
| :--- | :--- | :--- |
| HTTP Route | `github.com/duxweb/runa/route` | HTTP host, routing, context, binding, responses, errors |
| OpenAPI | `github.com/duxweb/runa/openapi` | Multi-document OpenAPI integration |
| Cache | `github.com/duxweb/runa/cache` | Named cache pools, memory driver by default |
| Queue | `github.com/duxweb/runa/queue` | Jobs, workers, retry, memory driver by default |
| Database | `github.com/duxweb/runa/database` | Named database runtime and driver registry |
| Oro Driver | `github.com/duxweb/runa/database/oro` | Official Oro ORM database driver |
| Storage | `github.com/duxweb/runa/storage` | Named disks, local driver by default |
| Session | `github.com/duxweb/runa/session` | Cookie/session stores and HTTP middleware |
| Auth | `github.com/duxweb/runa/auth` | Authenticators and auth middleware integration |
| View | `github.com/duxweb/runa/view` | Renderer registry and request-scoped template functions |
| Lang | `github.com/duxweb/runa/lang` | i18n catalogs, translators, locale matching |
| Task | `github.com/duxweb/runa/task` | Named task registry |
| Event | `github.com/duxweb/runa/event` | Event dispatch and listeners |
| Schedule | `github.com/duxweb/runa/schedule` | Cron-style scheduled tasks |
| Observe | `github.com/duxweb/runa/observe` | Trace/metric hooks and observability integration |

Driver modules keep heavier dependencies isolated, for example `cache/redis`, `queue/redis`, `queue/amqp`, `storage/s3`, `message/nats`, and `observe/prometheus`.

## Beyond HTTP

Runa's kernel is transport-neutral. `route` is the HTTP transport, `ws` adds WebSocket support, `jsonrpc` adds JSON-RPC support, and gRPC is planned as a future transport.

## Documentation

- English docs: <https://duxweb.github.io/runa/>
- Chinese docs: <https://duxweb.github.io/runa/zh-cn/>
- Naming conventions: `docs/src/content/docs/contributing/naming.mdx`
- Module boundaries: `docs/src/content/docs/contributing/modules.mdx`

## Contributing

Read `CONTRIBUTING.md` before opening a pull request. Keep dependencies explicit, update tests for behavior changes, and update English and Chinese docs together for public-facing changes.

## License

Runa is released under the MIT License. See `LICENSE`.
