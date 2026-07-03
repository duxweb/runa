<p align="center">
  <h1 align="center">Runa</h1>
</p>

<p align="center">
  <b>适用于业务开发的 Go Web 框架。</b><br/>
  从轻量微内核出发，按需装载能力，小到一条路由，大到企业级全栈应用，都能自由扩展。
</p>

<p align="center">
  <a href="https://duxweb.github.io/runa/zh-cn/">中文文档</a> &middot;
  <a href="https://duxweb.github.io/runa/">English Docs</a> &middot;
  <a href="#快速开始">快速开始</a> &middot;
  <a href="https://github.com/duxweb/runa/issues">反馈</a>
</p>

<p align="center">
  <a href="README.md">English</a> | 简体中文
</p>

<p align="center">
  <a href="https://github.com/duxweb/runa/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/duxweb/runa/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/duxweb/runa/actions/workflows/docs.yml"><img alt="Docs" src="https://github.com/duxweb/runa/actions/workflows/docs.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/duxweb/runa"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/duxweb/runa.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-black.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27rc1+-00ADD8.svg">
</p>

---

## 为什么是 Runa

| 常见问题 | Runa 的做法 |
| :--- | :--- |
| 小项目只想要 HTTP，大项目又需要完整能力 | 从 `runa` + `route` 起步，需要缓存、队列、数据库、认证、存储、视图、i18n、观测和驱动时再安装。 |
| 全栈框架容易带上没用到的依赖 | 每个能力都是独立 Go 模块，不 import 就不会进入编译路径。 |
| Router-first 框架让队列、命令和后台任务像附加能力 | 内核统一管理生命周期、DI、配置、命令、业务模块和 Host，HTTP 只是一个可插拔传输。 |
| 业务开发不只需要 handler | Runa 提供模块、结构化路由、显式验证、资源路由、CRUD、OpenAPI、队列、调度和视图集成。 |

## 状态

Runa 目前是 **pre-1.0 公开预览版**。当前公开版本目标是 `v0.1.3`，在 `v1.0` 前 API 仍可能调整。

当前最低 Go 版本是 **Go 1.27rc1**。在 Go 1.27 稳定版可用前，请使用 Go 1.27 RC 工具链。

## 安装

按需安装模块：

```bash
go get github.com/duxweb/runa
go get github.com/duxweb/runa/route
```

## 快速开始

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

运行：

```bash
go run .
curl http://localhost:8080/
```

## 架构

Runa 分为四层：

```text
内核层      github.com/duxweb/runa
传输层      route / ws / jsonrpc / future grpc
能力层      cache / queue / database / storage / auth / session / view / lang ...
驱动层      redis / s3 / amqp / nats / oro ...
```

根模块只提供应用启动、依赖注入、配置、命令、模块、生命周期、Host、错误和应用时间；默认不绑定 HTTP、缓存、数据库或队列。

## 能力

| 能力 | 安装路径 | 用途 |
| :--- | :--- | :--- |
| HTTP Route | `github.com/duxweb/runa/route` | HTTP Host、路由、Context、绑定、响应、错误处理 |
| OpenAPI | `github.com/duxweb/runa/openapi` | 多文档 OpenAPI 集成 |
| Cache | `github.com/duxweb/runa/cache` | 命名缓存池，默认 memory 驱动 |
| Queue | `github.com/duxweb/runa/queue` | Job、Worker、重试，默认 memory 驱动 |
| Database | `github.com/duxweb/runa/database` | 命名数据库运行时和驱动注册表 |
| Oro Driver | `github.com/duxweb/runa/database/oro` | 官方 Oro ORM 数据库驱动 |
| Storage | `github.com/duxweb/runa/storage` | 命名磁盘，默认 local 驱动 |
| Session | `github.com/duxweb/runa/session` | Cookie/session 存储和 HTTP 中间件 |
| Auth | `github.com/duxweb/runa/auth` | 认证器和鉴权中间件集成 |
| View | `github.com/duxweb/runa/view` | Renderer 注册表和请求级模板函数 |
| Lang | `github.com/duxweb/runa/lang` | i18n 语言包、翻译器、locale 匹配 |
| Task | `github.com/duxweb/runa/task` | 命名任务注册表 |
| Event | `github.com/duxweb/runa/event` | 事件派发和监听器 |
| Schedule | `github.com/duxweb/runa/schedule` | Cron 风格调度任务 |
| Observe | `github.com/duxweb/runa/observe` | Trace/Metric 钩子和观测集成 |

驱动模块会隔离重依赖，例如 `cache/redis`、`queue/redis`、`queue/amqp`、`storage/s3`、`message/nats`、`observe/prometheus`。

## 不止 HTTP

Runa 内核与传输无关。`route` 是 HTTP 传输，`ws` 提供 WebSocket，`jsonrpc` 提供 JSON-RPC，gRPC 作为后续传输规划中。

## 文档

- 中文文档：<https://duxweb.github.io/runa/zh-cn/>
- English docs: <https://duxweb.github.io/runa/>
- 命名规范：`docs/src/content/docs/zh-cn/contributing/naming.mdx`
- 模块边界：`docs/src/content/docs/zh-cn/contributing/modules.mdx`

## 贡献

提交 PR 前请先阅读 `CONTRIBUTING.md`。请保持依赖显式，行为变化要补测试，对外文档变化要同步中英文。

## 许可证

Runa 使用 MIT 许可证发布，见 `LICENSE`。
