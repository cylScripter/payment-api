# 任务清单 — 商户 API_KEY 接口鉴权

**输入** [spec.md](spec.md)、[plan.md](plan.md)、[research.md](research.md)、[data-model.md](data-model.md)、[HTTP 契约](contracts/http-api.md)、[quickstart.md](quickstart.md)

**日期** 2026-11-17 | **Git 分支** `main`

## 格式与执行约定

任务采用 `- [ ] T### [P?] [US?] 描述及文件路径` 格式。[P] 表示前置条件满足后可与同组的不同文件任务并行，用户故事阶段使用 [US1]、[US2]、[US3] 标识。

围绕现有项目实现，函数参数一般最多 3 个，查询请求采用私有字段与 option 构造。每次修改已有代码符号前执行 GitNexus upstream 影响分析，向用户报告直接依赖、流程及风险，高风险结果须先告知。保留开始实施时已有的修改。

本清单将独立 `internal/middleware` 包作为 HTTP 权限判定入口，商户号通过原有请求参数传递。中间件校验通过后放行，业务调用继续使用现有请求类型。验证采用构建、静态检查及联调指南中的场景。

## 阶段 1 — 准备

- [X] T001 核对 `AGENTS.md`、`.specify/memory/constitution.md` 和 `specs/001-api-key-auth/plan.md`，记录 `git status` 中的已有修改，确认本次涉及 Auth、Config、router 等符号的影响分析范围，特别记录 Config 的 CRITICAL 风险及回归流程。
- [X] T002 核对 `go.mod`、`Makefile` 及 `api/auth/v1/auth.proto` 的现有工具链，确认 Go 版本、protoc 和项目 protobuf 插件可用，依照现有 `make tools`、`make proto` 入口准备生成环境。

## 阶段 2 — 公共基础

本阶段提供各用户故事共用的协议、数据、认证能力与 HTTP 中间件基础。

- [X] T003 [P] 在 `api/auth/v1/auth.proto` 的现有 AuthService 中增加 `GenerateAPIKey(GenerateAPIKeyReq) returns (GenerateAPIKeyResp)` 及 HTTP 注解 `/v1/auth/api-keys`，请求包含 `merchant_account = 1`，响应包含 `api_key = 1`、`merchant_account = 2`，JSON 名明确为 snake_case。执行 `make proto` 更新 `api/auth/v1/auth.pb.go`、`api/auth/v1/auth_grpc.pb.go`。
- [X] T004 [P] 在 `internal/models/pay_merchant_api_key.go` 定义 PayMerchantAPIKey 及 TableName，在 `migrations/20261117_pay_merchant_api_key.sql` 和 `migrations/init.sql` 同步表结构。字段按数据模型约束设置，ID 为 BIGINT“自增主键”，MerchantID 为 BIGINT NOT NULL“关联现有商户 ID，普通索引”，KeyHash 为 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL“完整密钥的 SHA-256 十六进制摘要，唯一索引”，CreateTime 为 DATETIME(3) NOT NULL“创建时间，GORM autoCreateTime”，所有模型字段添加明确 gorm 标签。
- [X] T005 [P] 在 `internal/config/config.go` 增加 Config.Auth 和 AdminKey，在 `configs/config.yaml` 增加脱敏的 `auth.admin_key` 配置项。遵守“非空值必须为单个不含空白及逗号的凭证”“空值允许启动，签发接口返回 503”“热更新格式错误时保留旧快照”，在发布快照及执行配置应用副作用前完成字段校验，错误信息使用固定文本。
- [X] T006 在 `internal/repo/merchant_api_key.go` 增加持有私有 db 的 MySQLAPIKey、NewMySQLAPIKey 及 ReqAPIKey、ReqCreateAPIKey，按私有字段和 `opts ...func(*T)` 构造请求。实现按摘要识别已签发密钥及读取关联商户 ID、账号、状态的查询，传递 context 并使用 `dbresolver.Write`，保留记录不存在与数据库故障的区别，支持管理授权识别商户凭证。依赖 T004。
- [X] T007 在 `internal/middleware/api_key.go` 实现业务密钥认证与管理授权。业务认证按完整密钥摘要查主库，未知密钥、商户删除或禁用返回 Unauthenticated，数据库故障返回 Unavailable。管理授权接收协议入口传入的 Authorization 值并解析唯一 Bearer 凭证，使用 `crypto/subtle.ConstantTimeCompare` 比对当前配置管理密钥的固定长度摘要，已签发商户凭证返回 PermissionDenied，未知凭证返回 Unauthenticated，空管理配置返回 Unavailable，取消和超时沿用原有 context 映射。依赖 T005、T006。
- [X] T008 在 `internal/middleware/api_key.go` 实现独立 Auth 与 AdminAuth 中间件，接收 Controller.Response 作为错误响应回调。Auth 校验唯一 Bearer 值、密钥和商户状态，checkMerchant 参数控制请求商户号校验。AdminAuth 直接读取 Authorization 请求头并调用共用管理校验，设置 Cache-Control: no-store。权限失败立即 Abort，通过后 Next。依赖 T007。

## 阶段 3 — US1 为商户生成调用密钥（P1）

**目标** 平台通过管理凭证为指定正常商户取得首把及后续密钥。

**独立验收** 按联调指南完成 HTTP 和 RPC 首次签发、再次签发及未授权拒绝。成功返回 35 字符（sk- 加 32 位小写十六进制）密钥及正确商户账号，失败结果符合契约，未授权请求的密钥记录增量为 0。

- [X] T009 [P] [US1] 在 `internal/repo/merchant_api_key.go` 实现主库签发事务，按 BINARY account 精确匹配，最多读取两条商户并锁定目标行，复用商户状态“1 正常，2 禁用”。仅在匹配唯一且状态正常的商户时签发，插入摘要并在提交成功后返回绑定商户，失败直接返回 error。依赖 T006。
- [X] T010 [P] [US1] 在 `internal/controllers/api_key.go` 新增 GenerateAPIKey handler，按已生成的方法签名完成 protojson 请求绑定，使用 `ctx.Request.Context()` 调用现有 Auth 并通过 Controller.Response 返回 GenerateAPIKeyResp，绑定选项与现有 controller 一致。依赖 T003、T007，使用生成接口契约，可与 T009 并行。
- [X] T011 [US1] 在 `internal/service/auth.go` 实现 Auth.GenerateAPIKey，校验请求商户号“必填，非纯空白，最长 128 字符，与既有商户账号精确匹配”。使用 crypto/rand 生成 16 字节，编码为 32 位小写十六进制并加上 sk- 前缀，得到 35 字符密钥，摘要传给签发事务，提交成功后构造 GenerateAPIKeyResp。签发失败统一返回 503 和固定错误消息。依赖 T003、T007、T009。
- [X] T012 [US1] 在 `internal/router/router.go` 使用 Gin 分组，public 组注册健康检查与支付回调，merchant 组挂载 Auth(false)，payments 组挂载 Auth(true)，admin 组挂载 AdminAuth 并注册 POST /v1/auth/api-keys 到新增 handler，在 `internal/rpc/server.go` 的签发方法入口调用同一管理校验。依赖 T008、T010、T011。
- [X] T013 [US1] 按 `specs/001-api-key-auth/quickstart.md` 的首次签发、管理授权及 RPC 签发步骤联调，覆盖首把、重复签发、未知凭证、商户凭证访问管理接口、空账号、超长账号、不存在或禁用商户以及重复账号。核对成功响应包含正确账号和 35 字符（sk- 加 32 位小写十六进制）密钥，持久化记录包含 64 字符摘要，以新旧密钥调用空 code 登录请求得到原业务参数错误 400，验证均能立即通过认证，拒绝签发请求记录增量为 0，记录实际结果。依赖 T012。

## 阶段 4 — US2 携带密钥访问业务接口（P1）

**目标** 中间件完成商户身份及请求商户号的权限校验，通过后将原请求交给业务处理。

**独立验收** 有效密钥请求进入原业务流程，错误凭证返回 401，跨商户请求返回 403，拒绝请求的业务写入为 0。合法下单经过中间件后可以完整绑定请求。已交付密钥在重启和另一实例上可用。

- [X] T014 [P] [US2] 在 `internal/middleware/api_key.go` 增加 `POST /v1/payments/jsapi` 的请求商户权限检查，读取原始 body，通过 encoding/json 读取通用 merchant_account 参数并兼容 merchantAccount，商户号缺失或无效时返回 InvalidArgument，将账号与密钥所属商户账号精确比对，业务字段由 controller 绑定。错误格式返回 InvalidArgument，归属不符返回 PermissionDenied，非空账号的缓存商户 ID 与认证商户 ID 不一致返回 Unavailable，通过后以 `io.NopCloser(bytes.NewReader(body))` 恢复原始请求体再放行。商户号按原请求字段传递，微信登录执行密钥和商户状态认证。依赖 T008、T012。
- [X] T015 [P] [US2] 在 `internal/router/router.go` 使用 Gin 现有 LoggerConfig.SkipQueryString 避免查询串中的凭证写入访问日志，核对现有 recovery 对 Authorization 的遮蔽，复用原日志与恢复调用风格。依赖 T012，可与 T014 并行。
- [ ] T016 [US2] 按 `specs/001-api-key-auth/quickstart.md` 联调缺失、空值、错误方案、重复头、多凭证、未知密钥及仅查询参数或请求体携带密钥的拒绝场景；核对 Bearer 大小写规则、`merchant_account` 与 `merchantAccount` 的跨商户拒绝、重复字段处理，以及合法请求恢复 body 后的完整下单。确认拒绝请求进入业务 handler 的次数及新增业务记录均为 0，记录结果。依赖 T014、T015。
- [X] T017 [US2] 按 `specs/001-api-key-auth/quickstart.md` 完成连续 100 把及并发签发密钥的互异性与可用性核对，验证首把密钥、服务重启、同部署另一实例、商户禁用与恢复、商户删除后同账号重建、主库故障及响应丢失后重新签发。确认即时认证、旧密钥的主键归属、故障 503 及拒绝时业务写入为 0，记录结果。依赖 T016。

## 阶段 5 — US3 探活与支付通知正常接入（P2）

**目标** 健康检查和渠道回调按精确路由豁免 API_KEY 校验。

**独立验收** 无密钥访问健康检查取得现有结果，有效及无效渠道回调均按现有渠道规则处理，其他匹配业务路由接受中间件鉴权。

- [X] T018 [US3] 对照 `internal/router/router.go`、`internal/middleware/api_key.go` 和 `specs/001-api-key-auth/contracts/http-api.md` 核对路由分组及中间件挂载，健康检查与支付回调位于公开组，商户及支付接口位于对应受保护组，签发接口位于管理组。验证公开分组可直接处理原始回调请求、未知路由 404，以及新增公开接口可通过加入 public 组接入。依赖 T012、T014。
- [ ] T019 [US3] 按 `specs/001-api-key-auth/quickstart.md` 使用无 API_KEY 的健康检查、有效及无效渠道回调样本完成联调，核对现有处理与应答规则及相邻路径的权限行为，记录结果。依赖 T018。

## 阶段 6 — 验证与交付

- [X] T020 依据 `Makefile` 重新执行 `make proto`，对 `internal/config/config.go`、`internal/models/pay_merchant_api_key.go`、`internal/repo/merchant_api_key.go`、`internal/service/auth.go`、`internal/middleware/api_key.go`、`internal/controllers/api_key.go`、`internal/router/router.go`、`internal/rpc/server.go` 执行 gofmt，运行 `go build ./...` 和 `go vet ./...` 并修复发现的问题。依赖全部故事任务。
- [X] T021 按 `specs/001-api-key-auth/quickstart.md` 联调 `internal/config/config.go` 的管理凭证切换、空配置、错误类型及非空凭证格式错误，验证新快照生效与失败时继续使用旧快照，核对已有商户密钥可用性，以及现有 gRPC 业务调用和新增签发 RPC 的管理授权结果。
- [X] T022 对照 `specs/001-api-key-auth/spec.md` 的 SC-1 至 SC-9 和 `specs/001-api-key-auth/quickstart.md` 汇总验收结果，核对日志、错误响应和普通业务响应中完整密钥出现次数为 0，签发响应含 no-store，摘要入库及所有拒绝场景的业务副作用符合要求，说明实际环境和待验证项。
- [X] T023 核对 `specs/001-api-key-auth/plan.md` 中的最终改动范围，执行 GitNexus detect_changes 及 `git diff --check`，区分实施产生的变更与 T001 记录的已有修改，确认涉及符号与执行流程符合本功能，并汇总构建、静态检查、联调及剩余事项。

## 依赖顺序

```text
T001 → T002 → 公共基础 T003 至 T008
                         ↓
             US1 T009 ∥ T010 → T011 → T012 → T013
                                              ↓
                              T014 ∥ T015 → T016 → T017 ─┐
                                ↓                        │
                              T018 → T019 ───────────────┤
                                                         ↓
                                       T020 → T021 → T022 → T023
```

T003、T004、T005 在 T002 后可并行，基础阶段在 T008 后完成。T009 与 T010 分别实现 repo 事务和 HTTP handler，两者遵循 T003 已生成的接口签名，T011 提供实际签发实现。T012 等待这三个任务全部完成。

US1 先形成可联调的签发闭环，US2 使用已签发密钥验证业务授权。US3 的路由审阅依赖 T012、T014，可与 US2 的后续持久化场景联调分别开展，完整交付需汇合所有故事结果。

## 并行执行示例

| 阶段或故事 | 可并行内容                                | 汇合条件                                |
|-------|--------------------------------------|-------------------------------------|
| 公共基础  | T003 proto、T004 模型与 SfazQL、T005 配置   | T006 使用 T004 模型，T007 等待 T005 和 T006 |
| US1   | T009 repo 签发事务与 T010 新增 HTTP handler | T011 完成签发实现后，T012 接入路由              |
| US2   | T014 商户参数权限校验与 T015 访问日志配置           | T016 在二者完成后执行                       |
| US3   | T018 完成后，T019 中的探活与渠道回调场景可独立发起       | 统一汇总豁免及渠道处理结果                       |

相同文件的任务顺序执行，例如 T007 与 T011、T008 与 T014、T012 与 T015。运行验证时使用准备好的测试环境与商户数据。

## 实施策略

1. 首先完成准备与公共基础，形成统一协议、摘要存储和认证入口。
2. MVP 联调里程碑为 US1，交付平台管理凭证签发首把商户密钥的闭环。
3. US2 完成中间件的请求参数权限判定、原始 body 恢复，以及重启和跨实例可用性验证。
4. US3 完成健康检查及渠道回调豁免验收，最后执行构建、静态检查、配置回归与规格逐项核对。

## 需求与验收映射

| 需求或验收 | 主要任务 |
| --- | --- |
| FR-1、FR-2、FR-3，SC-1、SC-6 | T003、T005、T007、T009 至 T013 |
| FR-4、FR-6、FR-7、FR-8，SC-2、SC-3、SC-8 | T007、T008、T014、T016 |
| FR-9、FR-10、FR-13，SC-4、SC-9 | T004、T006、T007、T009、T011、T017 |
| FR-5，SC-5 | T008、T018、T019 |
| FR-11，SC-7 | T004、T011、T015、T022 |
| FR-12，原有调用及热更新回归 | T003、T012、T014、T020、T021 |

共 23 项任务，US1 为 5 项，US2 为 4 项，US3 为 2 项，公共准备、基础与交付验证为 12 项。

## 实施验收进度

隔离 MySQL 8.4 的实际验证结果见 quickstart.md 的本次实施验证记录。T016 的凭证、越权及请求体恢复场景已通过，完整渠道下单验收待渠道测试环境。T019 的公开组、健康检查及回调字节透传已通过，有效签名渠道回调验收待样本与测试渠道。两项保留未完成状态。
