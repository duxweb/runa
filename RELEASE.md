# v0.1.4 Release Notes

发布日期：2026-10-23。

Go 基线：`go 1.27.3`（正式版）。

Module：`github.com/duxweb/runa`。所有公开能力与驱动模块同步发布 `v0.1.4`，Oro 适配器依赖 `github.com/duxweb/oro v0.1.3`。

本次更新包含调度下一次执行时间展示、调度时区与异常配置修复、CRUD 滚动分页修复，以及生成器依赖版本修复。

完整变更见 [English changelog](CHANGELOG.md) 和 [中文更新日志](CHANGELOG.zh-CN.md)。

发布校验：所有工作区模块的测试与 vet 通过，Oro 适配器的 race 测试通过，文档构建通过。MySQL/PostgreSQL 集成测试因本机测试数据库不可用而跳过。
