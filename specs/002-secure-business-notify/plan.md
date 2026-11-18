# 实现计划 — 商户支付回调签名

**实际 Git 分支** `main` | **日期** 2026-11-18 | **规格** [spec.md](spec.md)

功能目录为 `specs/002-secure-business-notify`，初始化脚本返回的 `BRANCH` 为 `002-secure-business-notify`，当前 Git 分支通过 `git branch --show-current` 核实为 `main`。

## 概述

商户表保存独立回调密钥的密文，由本地运维命令初始化并向授权操作者输出明文用于交付。业务通知发送方法复用现有商户对象、JSON 报文和 HTTP 客户端，按用户明确的 HMAC-SHA256 规则增加时间戳及签名。商户按照原始报文验签、核对订单并幂等处理，详细契约和联调步骤分别记录在 [contracts/business-notify.md](contracts/business-notify.md) 与 [quickstart.md](quickstart.md)。

## 技术上下文

- **语言与版本** Go 1.26.0，以当前 `go.mod` 为依据，HTTP 使用 Gin，RPC 使用 gRPC。
- **存储** MySQL/GORM，新增 `pay_merchant.notify_secret` 密文字段，增量 SQL 和初始化 SQL 同步维护。
- **密钥能力** 复用 `helpers.GenerateSecret`、`EncryptPEMString`、`DecryptPEMString` 和 `config.Get().EncryptKey`。
- **签名能力** 标准库 `crypto/hmac`、`crypto/sha256`、`encoding/hex`、`strconv` 和 `time`。
- **现有外部交互** `sendBusinessNotify` 单次 HTTP 超时 2 秒，拒绝跟随跳转，以 HTTP 2xx 确认投递成功。商户按本规格返回 200 或 204。
- **配置读取** 通知入口已读取商户对象，补充调用方上下文和主库读取以获得已初始化的密钥。
- **验证** `go build ./...`、`go vet ./...`，现有加解密测试按需运行，固定签名向量及商户接收端联调覆盖协议和业务结果。
- **规划状态** 密钥分配入口、存储位置、并发初始化、签名位置及验证方式均已在 [research.md](research.md) 定案。

## 宪章检查

| 原则      | 规划前检查                    | 设计后复核                                                                  |
|---------|--------------------------|------------------------------------------------------------------------|
| 模块职责    | 确认现有商户、repo、通知模块分工       | 命令负责输入输出，service 组织密钥分配，repo 执行持久化，通知模块发送签名报文                          |
| 契约与兼容性  | 明确用户要求新增签名头              | 业务报文沿用现有字段与金额单位，商户接入文档补充验签约定                                           |
| 简洁与复用   | 已核对现有 helpers 与标准库       | 发送方法直接签名，密钥初始化复用现有数据库入口及配置加载                                           |
| 参数数量与构造 | 发送方法限制为三个参数              | 使用 `sendBusinessNotify(ctx, merchant, payload)`，新增 repo 请求沿用 option 构造 |
| 数据一致性   | 明确重复初始化必须复用已有密钥          | 主库读取、按主键条件写入空字段、读回最终值，保持单行初始化一致性                                       |
| 校验范围    | 来自规格的密钥、时间戳、签名和业务核对要求已列明 | 校验限定在执行命令所需账号及已明确的安全协议范围                                               |
| 验证与交付   | 明确构建、静态检查和协议验证路径         | 验证指南列出可执行命令与联调预期，功能验收在实现阶段执行                                           |

两轮检查通过，设计符合项目宪章与本轮明确需求。

## 涉及模块

```text
cmd/notify-secret/main.go                 本地密钥分配命令
internal/models/pay_merchant.go          商户密文字段及 JSON 排除标签
internal/repo/merchant.go                条件初始化、主库读回和通知读取上下文
internal/service/merchant/notify.go      密钥生成、加解密及分配流程
internal/service/notify/business.go      原始报文签名及请求头
internal/service/notify/wechat.go        传递完整商户对象及读取上下文
migrations/init.sql                     新建数据库的商户字段
migrations/20261118_merchant_notify_secret.sql  存量数据库增量字段
docs/payment-integration.md              商户回调验签说明
```

## 复用与约定

- 商户密钥通过 `PayMerchant.NotifySecret` 保存密文，使用 `json:"-"`，普通商户 JSON 和鉴权 Redis 缓存的序列化自动排除该字段。
- 复用 `MerchantMysql` 作为已有商户数据库入口。新增初始化查询请求使用 `ReqNotifySecret`、`NewReqNotifySecret(opts ...func(*ReqNotifySecret))`，选项承载商户账号和候选密文。
- 本地命令只初始化配置和数据库，使用 `config.Init()`、`initials.InitDatabase(config.Get())` 及现有连接池关闭入口。
- 商户服务中的 `NotifySecret(ctx, account)` 返回已保存密钥的明文供本地命令交付。先取得商户记录，已有密文则解密返回，空值才生成并加密候选密钥。
- repo 按商户主键与空字段条件初始化，并从主库读回最终密文。并发调用的返回结果均来自实际保存值，命令中断后重试可取得原密钥。
- `OneMerchant` 补充 `context.Context` 并使用主库查询，通知读取沿用原有错误处理习惯。
- `sendBusinessNotify` 接收 `ctx`、商户对象、业务报文三个参数，在单个方法内完成密钥解密、一次序列化、签名及发送。
- HMAC key 使用商户密钥字符串的 UTF-8 字节。每次发送读取当前时间，写入 `v1\n`、时间戳、换行、原始 body、末尾换行，再输出小写十六进制。
- 商户端执行时间、签名、订单核对及幂等规则。文档使用 `hmac.Equal` 比较十六进制解码后的字节。

## 实施顺序与状态

1. 同步商户字段和迁移，完成密钥分配服务与本地命令，核对顺序/并发重复初始化返回同一保存值。
2. 为测试商户初始化密钥，交付给商户测试接收端，完成固定向量验签。
3. 集成实际通知签名，在发送前已有的 `PENDING` 状态基础上返回投递错误，成功后沿用现有 `SUCCESS` 更新。
4. 更新商户文档，按验证指南核对真实发送原始字节、失败分支、时间窗口及商户幂等。
5. 发布顺序为数据库字段、目标商户密钥准备与接收端验签部署、支付服务签名代码部署。空密钥商户按规格产生投递失败，已确认的支付结果继续有效。

新增安全信息承载在请求头中。密钥的状态变化及通知投递状态见 [data-model.md](data-model.md)。

## 调用影响

GitNexus 对 `sendBusinessNotify` 的 upstream 分析结果为 LOW，直接调用方共一个，为 `WechatNotifier.NotifyBusiness`。索引中的受影响流程数量为 0，源码核对确认该方法用于微信支付成功后的业务通知步骤。`OneMerchant` 当前直接调用位于 `WechatNotifier.LoadConfig`。

实现阶段在修改每个现有函数或方法前重新执行相应 impact 分析。商户模型新增字段需要核对鉴权缓存 JSON、商户缓存和数据库读取。提交前执行 `detect_changes()` 核对实际变更范围。

## Phase 0 与 Phase 1 产物

- [research.md](research.md) 记录现成能力、分配入口、并发初始化、签名位置及验证决策。
- [data-model.md](data-model.md) 记录新增字段、数据流和状态变化。
- [contracts/business-notify.md](contracts/business-notify.md) 记录本地命令及商户回调契约和固定签名向量。
- [quickstart.md](quickstart.md) 记录构建、命令验证、协议核对和商户联调步骤。

## 复杂度说明

设计符合现有约定。已有随机密钥与加解密工具、商户 repo、通知客户端和标准库可支撑本次需求，新增能力集中在密钥持久化、分配命令和发送签名步骤。
