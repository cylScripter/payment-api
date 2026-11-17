---
description: "功能实现任务清单模板"
---

# 任务清单：[功能名称]

**输入**：`/specs/[###-feature-name]/` 下的 spec.md 和 plan.md

## 格式

`- [ ] T### [P?] [US?] 描述`

- **[P]**：可并行执行（不同文件、无依赖）
- **[US]**：所属用户故事（US1、US2...），公共基础任务省略
- 描述中写明具体文件路径
- 按执行依赖排序，按用户故事分组

## 阶段 1：接口定义与基础

- [ ] T001 修改/新增 proto 定义于 `api/<业务>/v1/`，执行 `make proto` 同步生成代码
- [ ] T002 [P] 数据模型及迁移 SQL：`internal/models/`、`migrations/`（gorm 标签、`TableName()`）

## 阶段 2：US1 - [标题]（P1）

- [ ] T003 [US1] 数据库访问：`internal/repo/`（`MySQLXxx` + `NewReqXxx(opts ...func(*ReqXxx))`）
- [ ] T004 [US1] 业务逻辑：`internal/service/`（复用已有 proto 请求响应类型，错误用 gRPC status）
- [ ] T005 [US1] controller 及路由注册：`internal/controllers/`、`internal/router/router.go`（或 `internal/rpc/server.go`）

## 阶段 3：US2 - [标题]（P2）

- [ ] T006 [US2] [具体任务及文件路径]

## 阶段 4：验证

- [ ] T### 执行 `go build ./...`、`go vet ./...`，gofmt 修改过的文件，修复问题
- [ ] T### 逐条核对 spec.md 的验收标准
