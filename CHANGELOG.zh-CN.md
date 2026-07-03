# 更新日志

[English](CHANGELOG.md) | 简体中文

## v0.1.3 - 2026-07-03

### 新增

- 新增 storage 列表能力，支持游标分页、递归列表和公共目录结果，local 与 S3 disk 均已实现。
- 新增 storage 原生 Copy/Move 扩展接口，并为 local filesystem 与 S3 提供实现。
- 新增 `storage/s3.Provider`，支持 client 注入、options 构造、`[storage.s3]` 配置、共享 `[s3]` / `[s3.<name>]` 配置，以及 AWS 默认凭证链回落。

### 变更

- 增强 S3 驱动：支持 multipart upload、S3 兼容端点的删除兼容行为、metadata 写入、按扩展名推断 Content-Type，以及服务端 CopySource URL 编码。
- 补充 MinIO、Cloudflare R2、阿里云 OSS、腾讯云 COS、七牛 Kodo 的 S3 兼容配置文档。

### 修复

- 对自定义 S3 兼容 endpoint 自动关闭不兼容的 AWS SDK checksum 行为。
- 修复 S3 copy/move 在 object key 包含空格、`+`、`#` 或非 ASCII 字符时的问题。

## v0.1.2 - 2026-07-02

### 变更

- 将 Redis queue 驱动的 job 存储从整段 JSON String 优化为 Redis Hash，状态流转只更新可变字段。
- 降低 Redis queue 热路径 RTT：Reserve 在 Lua 内完成 claim 与字段更新，Ack 和 Fail 改为单 Lua 脚本，Release 无需读取整段 body，List/Purge 使用批量 Hash 读取。
- 新增真实 Redis 基准测试，可通过 `RUNA_REDIS_BENCH_ADDR` 启用 queue push、reserve/ack 和 worker throughput benchmark。

### 修复

- 保留 Redis queue 坏 body 自愈能力，避免单条坏数据毒化整批 reserved job。
- 在 Redis job body 改为 Hash 存储的同时，保持 `queue.Driver`、`queue.JobMessage` 和 worker API 不变。

## v0.1.1 - 2026-07-01

### 变更

- 统一 `route.Context` 泛型访问器为 Go 1.27 泛型方法，包括 `Param`、`Query`、`Header`、`Cookie`、`Form`、`Meta`、`Input`、`Service`。
- 将路由元数据读取改为 `*route.Route` 上的 `route.MetaAs[T](...)` 方法。
- 将可选 XLSX CRUD 导入导出拆到 `crud/excelize`，基础 `crud` 模块不再携带 Excel 依赖。
- 将 message broker 接口统一命名为 `message.Driver`，并同步官方消息驱动。
- 通过 `core.ColorEnabled` 统一终端颜色检测。
- 抽取 `view` 与 `view/rhtml` 共享的模板函数辅助逻辑。

### 修复

- 删除陈旧的 `internal/registry` 副本，统一使用 `kernel/registry`。
- 移除冗余 helper 变体和重复的 `contains` 实现。
- 更新 route、CRUD、audit、middleware、resource、console 和代码生成模板，适配新的泛型方法风格。
- 更新中英文文档，覆盖 route context helper、CRUD Excelize、模块安装和注入 route service 的用法。

## v0.1.0 - 2026-07-01

### 新增

- 新增 Runa 微内核：应用启动、Provider 生命周期、Module 生命周期、DI、配置、命令、Host、应用时间和关闭流程。
- 新增 HTTP route 能力：结构化路由、请求上下文、绑定辅助、验证集成、响应辅助、错误渲染、路由分组、中间件和 HTTP Host 启动。
- 新增官方中间件模块：recover、request id、real ip、logger、CORS、CSRF、body limit、timeout、helmet、healthcheck、static、语言协商、session、auth、rate、audit 集成。
- 新增业务能力：cache、queue、database、storage、session、auth、view、language/i18n、event、task、schedule、message、lock、rate、asset、RBAC、sanitize、安全预设。
- 新增官方驱动和适配器：Redis、AMQP、MQTT、NATS、S3、Oro database、Oro CRUD store、Snowflake ID、WebSocket Redis、cluster Redis、Prometheus observe 集成。
- 新增 OpenAPI、资源路由、CRUD 辅助、RHTML 模板集成、WebSocket 传输、JSON-RPC 传输、console、audit、observe、devtools 和示例项目。
- 新增 `runa` CLI 工具：开发热重载、代码生成、信息 introspection、doctor 检查、MCP、`llms.txt` 生成。
- 新增 `docs/` 文档站源码、GitHub Pages workflow、CI workflow、发布脚本、MIT License、安全策略、贡献指南和 Issue/PR 模板。

### 已知限制

- 当前是 pre-1.0 发布候选，`v1.0` 前公开 API 仍可能变化。
- 当前最低 Go 版本是 `1.27rc1`。
- gRPC 传输仍是规划中，本版本未实现。
- 首个公开版本使用多模块 tag，每个子模块都可以独立按需安装。
