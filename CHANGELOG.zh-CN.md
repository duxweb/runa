# 更新日志

[English](CHANGELOG.md) | 简体中文

## v0.1.0 - 未发布

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
- 最终发布仍需要在发布验证通过后创建仓库 tag。
