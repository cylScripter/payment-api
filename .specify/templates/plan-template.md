# 实现计划：[功能名称]

**分支**：`[###-feature-name]` | **日期**：[DATE] | **规格**：[链接]

## 概述

[一段话：主要需求 + 技术方案。沿用项目现有分层和实现方式，不引入新的框架或抽象]

## 技术上下文

- **语言/版本**：Go（版本以 `go.mod` 为准），HTTP 用 Gin，RPC 用 gRPC
- **存储**：MySQL（GORM），迁移 SQL 放 `migrations/`
- **验证**：`go build ./...`、`go vet ./...`，按需运行现有测试

## 涉及模块

[列出本次实际会改动的目录，未涉及的删除]

```text
api/<业务>/v1/           # proto 定义（改动后执行 make proto）
internal/models/         # 数据模型（gorm 标签、TableName()）
internal/repo/           # 数据库访问（MySQLXxx + ReqXxx option 模式）
internal/service/        # 业务逻辑（HTTP 与 RPC 复用）
internal/controllers/    # HTTP 请求绑定与响应
internal/router/router.go / internal/rpc/server.go  # 路由与 RPC 注册
migrations/              # 数据库变更 SQL
```

## 复用与约定

[说明复用了哪些现有逻辑、类型和调用方式；沿用哪些既有写法。无特殊说明时写"按项目协作约定与宪章执行"]

## 复杂度说明

[仅在实现偏离项目约定时填写偏离点及原因，否则写"无"]
