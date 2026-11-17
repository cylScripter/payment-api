# 实现计划 — 商户 API_KEY 接口鉴权

**Git 分支** `main` | **功能标识** `001-api-key-auth` | **日期** 2026-11-17 | **规格** [spec.md](spec.md)

setup-plan 返回的 `BRANCH=001-api-key-auth` 是功能目录派生标识，实际 Git 分支经核对为 `main`。

## 概述

为已有正常商户提供 HTTP 密钥签发接口，使用独立管理凭证授权。商户使用 Bearer API_KEY 调用业务接口，密钥关联商户 ID，数据库保存 SHA-256 摘要。独立 middleware 包校验密钥、商户状态及请求参数中的商户账号，权限校验通过后放行。商户账号沿用现有请求参数传给业务处理。健康检查及支付回调按精确路由豁免。

## 技术上下文

- **语言与依赖** Go 1.26.0、Gin 1.12.0、GORM 1.31.2、DBResolver 1.6.2，使用现有 Protobuf 工具链。
- **存储** MySQL 新增 `pay_merchant_api_key` 表，增量及初始化 SQL 同步维护。
- **密钥** `crypto/rand.Read` 生成 32 字节，RawURL Base64 编码为 43 字符。完整密钥的 SHA-256 十六进制摘要作为唯一查询条件。
- **管理配置** 新增 `auth.admin_key`，通过 `config.Get()` 读取当前快照。缺省或空值允许服务启动，签发接口返回 503。非空值须为无空白、无逗号的单个凭证值，验证失败沿用当前配置。
- **一致性** 鉴权、商户状态读取及签发事务通过 `dbresolver.Write` 使用主库，所有查询传递调用方 context。
- **验证** 实施后执行 `make proto`、`go build ./...`、`go vet ./...` 及 [quickstart.md](quickstart.md) 的联调步骤。
- **研究结果** 见 [research.md](research.md)，技术决策已确定。

## 涉及模块

```text
api/auth/v1/auth.proto                       # 在现有 AuthService 中增加签发方法和消息，更新生成代码
internal/config/config.go                    # Auth 配置及发布快照前的字段校验
internal/models/pay_merchant_api_key.go       # 密钥摘要模型
internal/repo/merchant_api_key.go             # 主库查询和签发事务
internal/service/auth.go                     # 在现有 Auth 中增加密钥签发与身份认证
internal/middleware/api_key.go               # HTTP 认证、请求商户权限校验、路由豁免及放行
internal/controllers/api_key.go              # 新增签发接口的请求绑定与响应
internal/router/router.go                    # 全局鉴权、签发路由、访问日志配置
migrations/20261117_pay_merchant_api_key.sql   # 增量表结构
migrations/init.sql                          # 同步新表定义
configs/config.yaml                          # auth.admin_key 配置项
```

### HTTP 调用流程

1. `router.New` 在现有日志及恢复处理后，通过 `middleware.Auth(controller.Response)` 挂载独立包中的统一认证中间件。复用响应方法作为回调传入。依据 HTTP 方法和 `ctx.FullPath()` 匹配健康检查、支付回调、签发接口，其余已匹配业务路由默认使用商户鉴权。未匹配路径沿用现有 404 响应。
2. 中间件解析唯一 Authorization 值。签发路由将该值通过 `metadata.NewIncomingContext` 放入请求 context 的 `authorization` 元数据，并调用 `service.Auth` 的管理认证方法。业务路由调用 `service.Auth.Authenticate(ctx, key)`，取得密钥所属商户及当前状态。认证失败立即终止请求，通过传入的回调输出统一错误响应。
3. 对 `POST /v1/payments/jsapi`，中间件读取原始请求体，使用现有 `paymentv1.CreateOrderReq` 和 `protojson.UnmarshalOptions{DiscardUnknown: true}` 解析参数。将非空 `req.MerchantAccount` 与密钥所属商户账号精确比对，不一致返回 `PermissionDenied`。格式错误返回 `InvalidArgument`，商户账号为空时沿用业务接口的原有规则。
4. 权限通过后，以 `io.NopCloser(bytes.NewReader(body))` 将原始字节恢复到 `ctx.Request.Body`，调用 `ctx.Next()` 放行。controller 按现有流程绑定请求并将 `req.MerchantAccount` 作为请求参数传给业务 service。微信登录只需校验密钥及商户状态。后续新增带商户字段的 HTTP 路由在中间件补充对应 proto 请求类型和参数比对分支。
5. 在 `api/auth/v1/auth.proto` 的现有 `AuthService` 中增加 `GenerateAPIKey(GenerateAPIKeyReq) returns (GenerateAPIKeyResp)`，请求和响应消息定义在同一文件。HTTP 显式注册 `/v1/auth/api-keys`，controller 使用请求 context 调用 `service.Auth.GenerateAPIKey`。
6. 现有 AuthService 注册会同时提供 `/payment.auth.v1.AuthService/GenerateAPIKey`。该 service 方法在签发前统一从 incoming metadata 读取并校验管理凭证，覆盖 HTTP 和 RPC 两个入口。HTTP 中间件的预校验保证凭证错误在请求体绑定前返回。现有 gRPC 业务接口沿用其调用契约。

### 签发与持久化

生成原文及摘要后，repo 在主库事务内按账号读取并锁定目标商户，确认仅有一个匹配商户且状态为 1，随后插入摘要并提交。成功响应包含原文及商户账号。每次调用插入一条独立记录，唯一索引处理并发碰撞。密钥、归属关系和失败行为见 [data-model.md](data-model.md)。

### 错误与日志

沿用 `Controller.Response` 的映射，认证失败 401、权限不足 403、签发商户不存在 404、账号歧义或摘要冲突 409、数据库不可用 503。请求取消或超时沿用 context 对应状态。认证失败消息采用固定文本。

GORM 仅接收摘要。Gin Logger 使用现有 `LoggerConfig.SkipQueryString` 避免查询参数中的凭证进入访问日志，继续使用恢复组件对 Authorization 的遮蔽。新增代码中的日志和错误响应只使用固定消息、商户 ID 或账号。签发响应添加 `Cache-Control: no-store`。

## 复用与约定

- 复用商户模型、数据库入口、`ConfirmOrder` 的主库事务与行锁范式、原子配置快照、protojson 绑定及统一错误响应。
- 中间件位于 `internal/middleware`，根据现有 proto 请求参数完成 HTTP 权限判定，通过后放行。请求体读取后恢复原始字节，供后续 controller 绑定。业务 service 通过请求类型中的商户账号字段处理业务。
- 新 repo 使用 `MySQLAPIKey` 和 `NewMySQLAPIKey(db)`，查询及签发条件使用私有字段的 `ReqXxx` 与 `opts ...func(*T)` 构造，函数参数最多 3 个。
- 新方法返回查询 error，以区分未知密钥与数据库故障。原有忽略 error 的调用路径沿用现状。
- 标准库承担随机生成、摘要、编码与管理凭证比较。实施验证使用构建、静态检查及手工联调场景。

### 宪章检查

| 原则       | 设计前检查                                | 设计后检查                                                |
|----------|--------------------------------------|------------------------------------------------------|
| I 分层     | 通过，沿用 controller/service/repo/models | 通过，中间件负责 HTTP 权限判定，service 提供认证查询与签发，controller 绑定与应答，事务在 repo |
| II 契约与兼容 | 通过，扩展现有 AuthService 契约               | 通过，签发方法统一校验管理凭证，业务接口新增 HTTP 凭证要求，已有字段及金额约定延用         |
| III 简洁实现 | 通过，标准库与现有依赖足够                        | 通过，使用一个密钥表及现有生命周期，无新增后台任务                            |
| IV 数据访问  | 通过，复用 GORM 与现有商户主键                   | 通过，模型标签、TableName、repo 私有 db 及 option 查询结构已明确        |

### 静态影响与回归范围

GitNexus 对 `router.New` 的 upstream 分析为 LOW，1 个直接依赖为启动装配。回归重点覆盖路由豁免、HTTP 中间件权限判定、请求体恢复后下单处理及现有 gRPC 调用场景。实施时按最终涉及符号重新执行影响分析。

`config.Config` 的静态风险为 CRITICAL，3 个直接依赖、11 条受影响流程，涉及启动、热更新、数据库和渠道配置。实施时再次执行影响分析，保持配置字段添加式兼容，并验证旧配置启动、新旧管理凭证切换及无效热更新继续使用旧快照。

### 设计产物与实施顺序

研究、数据模型、HTTP 契约和联调指南已完成。后续任务按模型与迁移、repo、service、配置、HTTP 中间件与 controller、proto 生成和联调顺序拆分。对应契约见 [contracts/http-api.md](contracts/http-api.md)。

## 复杂度说明

按用户补充要求，HTTP 权限校验集中到独立的 `internal/middleware` 包，商户账号通过现有请求参数传递，中间件校验通过后放行。新增签发 HTTP handler 承担请求绑定与应答。签发方法和消息定义放入现有 `api/auth/v1/auth.proto` 的 AuthService，复用现有 Auth 实现。管理凭证签发、密钥长期有效及多密钥并存沿用规格中的默认假设。
