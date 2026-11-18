# 任务清单 — 商户支付回调签名

**输入** [spec.md](spec.md)、[plan.md](plan.md)、[research.md](research.md)、[data-model.md](data-model.md)、[回调契约](contracts/business-notify.md)、[验证指南](quickstart.md)

## 格式与执行约定

任务使用复选框、连续编号、可选 `[P]` 和用户故事标签。`[P]` 表示在满足本节依赖后可与其他独立文件任务并行。路径相对项目根目录。

修改现有函数、方法或类型前执行 GitNexus upstream impact，向用户报告调用方、流程及风险，HIGH 或 CRITICAL 风险在编辑前告知用户。实现前应用 Ponytail 技能并核对现有能力，沿用项目错误处理、上下文和 option 构造习惯。

验收结果写入 `specs/002-secure-business-notify/quickstart.md` 的验收记录，记录执行命令、结果和环境依赖。任务完成标记依据实际实现与验证结果，商户端结果通过接收端记录核实。

## 阶段 1 — 准备

**目标** 核对当前实现入口与设计一致，确认本次工作涉及的文件和调用关系。

- [X] T001 核对 `internal/pkg/helpers/crypto.go` 的随机密钥及加解密能力、`internal/repo/merchant.go` 的商户读取、`internal/service/notify/business.go` 和 `internal/service/notify/wechat.go` 的发送及状态流转，确认 `specs/002-secure-business-notify/plan.md` 中现成能力、三个参数签名和主库读取的实施位置。

## 阶段 2 — 公共数据基础

**目标** 为所有故事提供商户密文存储，模型与新旧数据库结构一致。

- [X] T002 [P] 在 `internal/models/pay_merchant.go` 为 `PayMerchant` 增加 `NotifySecret string`，字段约束严格采用 `column:notify_secret;type:varchar(512);not null;default:''` 和 `json:"-"`，保存 `EncryptPEMString` 生成的 `v1:` 格式密文，沿用现有 `TableName()`。
- [X] T003 [P] 在 `migrations/init.sql` 同步 `pay_merchant.notify_secret`，新增 `migrations/20261118_merchant_notify_secret.sql`，存储约束严格采用 `VARCHAR(512) NOT NULL DEFAULT ''`，使存量商户字段初值为空字符串。

**完成条件** 模型可编译，初始化结构与增量字段约束一致，普通商户 JSON 序列化结果排除 `notify_secret`。

## 阶段 3 — US1 取得独立回调密钥并验证通知（P1）

**目标** 商户取得稳定、独立的回调密钥，支付服务以实际发送报文生成可验证的通知。

**独立验收** 两个商户分别初始化密钥并取得有效签名通知，交叉密钥验签失败。同一商户重复及并发分配取得同一保存值，数据库保存密文，缺少可用密钥时投递失败且未签名请求送达数为 0。

- [X] T004 [US1] 在 `internal/repo/merchant.go` 的 `OneMerchant` 增加调用方 `context.Context` 并使用主库读取，在 `internal/service/notify/wechat.go` 的 `LoadConfig` 同步传入上下文，使通知读取已初始化的商户密钥，沿用现有商户加载错误处理方式。
- [X] T005 [US1] 在 `internal/repo/merchant.go` 补充主库商户查询和密钥条件初始化入口，查询请求采用 `ReqNotifySecret`、`NewReqNotifySecret(opts ...func(*ReqNotifySecret))` 和对应账号/候选密文选项；按已读取商户 `id` 与 `notify_secret = ''` 条件保存，再读回主库实际值，重复及并发初始化返回同一保存结果，数据库操作沿用调用方上下文。
- [X] T006 [P] [US1] 在 `internal/service/merchant/notify.go` 实现 `NotifySecret(ctx, account)`，已有密文时解密返回，空值时复用 `helpers.GenerateSecret` 生成“16 个安全随机字节的 32 字符十六进制表示”，使用 `EncryptPEMString`、`DecryptPEMString` 和 `config.Get().EncryptKey` 完成加解密，交付明文取自 repo 最终保存值，失败沿用 gRPC status 返回方式。
- [X] T007 [US1] 新增 `cmd/notify-secret/main.go`，接收 `-account`，复用 `config.Init()`、`initials.InitDatabase(config.Get())` 和现有连接池关闭入口，调用商户 service；成功向授权操作者的标准输出返回 `merchant_account`、`notify_secret` JSON 并退出 0，失败向标准错误输出原因并以非 0 退出，输出遵循 `specs/002-secure-business-notify/contracts/business-notify.md` 的交付契约。
- [X] T008 [P] [US1] 在 `internal/service/notify/business.go` 将发送方法改为 `sendBusinessNotify(ctx, merchant, payload)`，解密商户密钥，对一次 `json.Marshal(payload)` 得到的 body 按 `"v1\n" + timestamp + "\n" + body + "\n"` 计算 HMAC-SHA256，key 使用密钥字符串 UTF-8 字节，生成 64 个小写十六进制字符，添加 `X-Pay-Timestamp` 和 `X-Pay-Signature: v1=<signature>` 并发送同一 body；在 `internal/service/notify/wechat.go` 同步传递 `n.merchant`，密钥或解密失败时返回简单投递错误。
- [X] T009 [P] [US1] 更新 `docs/payment-integration.md` 的商户接入准备，说明取得独立 `notify_secret`、商户后端保存及其与 API Key 的用途，补充与商户账号关联的密钥使用约定。
- [ ] T010 [US1] 按 `specs/002-secure-business-notify/quickstart.md` 第 1、2、4 节执行密钥分配、顺序/并发重复分配、固定向量和实际发送原始字节核对，检查数据库密文、商户 JSON/鉴权缓存序列化隔离及缺密钥/不可解密时的失败结果，在该文件记录 US1 验收证据。

**完成条件** US1 的密钥分配与签名链路可独立使用，覆盖 FR-1～FR-5、FR-11 和 SC-1、SC-2、SC-7 的发送方行为。

## 阶段 4 — US2 拒绝篡改或过期的通知（P1）

**目标** 商户能按文档基于原始报文验签，拒绝错误来源、被修改、过期及业务信息不匹配的通知。

**独立验收** 正确密钥、原始报文和有效时间下验签成功。篡改报文/时间戳、错误密钥/凭证及超过时间窗口均拒绝业务处理，前后 300 秒通过时效校验、301 秒拒绝，业务账号、订单号和金额不匹配时业务处理次数为 0。

- [X] T011 [P] [US2] 更新 `docs/payment-integration.md` 的回调章节，写明两个签名头、`v1` 拼接规则与末尾换行、密钥 UTF-8 字节、秒级时间戳、小写十六进制及 `hmac.Equal` 比较解码后字节的验签示例；按“读取原始 body、校验 300 秒窗口、验签、解析 JSON、核对商户/订单/金额”的顺序说明，保留下单元金额和回调整数分约定，提供 `specs/002-secure-business-notify/contracts/business-notify.md` 中可复现的签名向量。
- [ ] T012 [P] [US2] 按 `specs/002-secure-business-notify/quickstart.md` 第 2、3、5 节使用本地协议核对及已配置商户测试接收端核实错误密钥、报文内容/空白修改、时间戳篡改、缺失/错误签名及不支持版本的处理，验证文档示例与实际通知字节一致，在该文件记录拒绝结果及业务处理次数。
- [ ] T013 [US2] 按 `specs/002-secure-business-notify/quickstart.md` 第 5 节，在商户测试接收端以固定接收时钟分别验证前后 300 秒与 301 秒边界，再用有效签名验证商户账号、业务订单号和订单总金额不匹配的情况，在该文件记录 US2 验收结果并对应 SC-3、SC-4 和 SC-7。

**完成条件** FR-6～FR-8 的商户接收约定与拒绝场景均有可复核结果，商户文档覆盖 FR-12 的签名与业务核对要求。

## 阶段 5 — US3 幂等处理重复通知和重新投递（P1）

**目标** 商户按支付单幂等处理有效通知，实际重新投递使用当前时间重新签名。

**独立验收** 同一支付单串行及并发各通知 10 次，业务效果累计为一次，有效重复通知返回 200 或 204。原通知过期后实际重新投递可在新窗口内验证，支付单号及交易时间含义符合原约定。

- [X] T014 [P] [US3] 更新 `docs/payment-integration.md` 的重复通知及成功应答说明，明确按 `out_trade_no` 幂等处理、处理成功和已成功处理的有效重复通知返回 HTTP 200 或 204，并说明通知时间与原支付成功时间的用途及重新投递的签名规则。
- [ ] T015 [P] [US3] 对照 `internal/service/notify/business.go`、`internal/service/notify/wechat.go` 和 `specs/002-secure-business-notify/quickstart.md` 第 4、5 节，捕获同一待通知支付结果在原请求过期后再次发送的时间戳与签名，核实每次实际发送使用当前时间重新计算、原支付单号和 `pay_time` 含义一致；同时验证非 2xx/超时保持 `PENDING`、成功应答后置 `SUCCESS`，在指南中记录结果。
- [ ] T016 [US3] 按 `specs/002-secure-business-notify/quickstart.md` 第 3、5 节在已配置商户测试接收端串行及并发各投递 10 次有效通知，并模拟业务处理成功后应答丢失的重复请求，核对每个支付单业务效果累计为一次、有效重复通知返回 200 或 204，在该文件记录 SC-5、SC-6 对应结果。

**完成条件** FR-9、FR-10 的商户幂等与发送方重新签名均完成验收，业务投递状态符合现有通知流程。

## 阶段 6 — 整体验证与交付

- [X] T017 对 `cmd/notify-secret/main.go`、`internal/models/pay_merchant.go`、`internal/repo/merchant.go`、`internal/service/merchant/notify.go`、`internal/service/notify/business.go`、`internal/service/notify/wechat.go` 中实际修改的 Go 文件执行 gofmt，在项目根目录执行 `go build ./...`、`go vet ./...` 及现有 `go test ./internal/pkg/helpers`，将命令和结果记录到 `specs/002-secure-business-notify/quickstart.md`。
- [X] T018 逐条核对 `specs/002-secure-business-notify/spec.md` 的 FR-1～FR-12 与 SC-1～SC-8，汇总 `specs/002-secure-business-notify/quickstart.md` 中的已验证结果与实际环境依赖，核实模型/迁移一致、密钥分配入口、商户文档和通知失败状态，并据实更新 `specs/002-secure-business-notify/tasks.md` 的完成标记。
- [X] T019 核对 `docs/payment-integration.md`、`specs/002-secure-business-notify/contracts/business-notify.md` 和实际实现契约，执行 `git diff --check` 并检查变更范围；准备提交时运行 GitNexus `detect_changes()`，在 `specs/002-secure-business-notify/quickstart.md` 记录最终验证结果和“字段迁移、商户密钥交付及接收端准备、签名代码部署”的发布顺序。

## 依赖关系

```text
T001
  -> T002、T003
  -> T004 -> T005
               -> T006 -> T007
               -> T008
        T009 可与密钥/发送实现并行
  -> T010（US1 验收）
  -> T011、T012 -> T013（US2 验收）
  -> T014、T015 -> T016（US3 验收）
  -> T017 -> T018 -> T019
```

- 公共基础 T002、T003 完成后进入用户故事阶段。
- US1 中 T004、T005 顺序执行，T006 完成后执行 T007，T008 在 T004、T005 后可与 T006、T007 并行，T009 可与上述实现并行，T010 汇合 US1 全部任务后执行。
- US2 依赖 US1 的可用密钥与签名通知。T011、T012 可并行，T013 使用两者完成后的接收约定和验证结果继续验收。
- US3 依赖 US1 的实际发送链路及 US2 的有效通知判定。T014、T015 可并行，T016 在 T015 后执行。
- 各故事的验收记录写入同一 `quickstart.md`，按上述阶段顺序汇总。

## 并行执行示例

| 阶段或故事 | 可并行任务            | 前置条件与文件归属                               |
|-------|------------------|-----------------------------------------|
| 公共基础  | T002 与 T003      | T001 完成，分别修改模型和 SQL                     |
| US1   | T006 与 T008、T009 | T004、T005 完成，分别修改商户 service、通知发送文件和商户文档 |
| US2   | T011 与 T012      | US1 完成，分别修改商户文档和验证记录                    |
| US3   | T014 与 T015      | US2 完成，分别修改商户文档和验证记录                    |

## 需求与任务对应

| 需求           | 主要任务                          |
|--------------|-------------------------------|
| FR-1         | T002、T003、T005、T006、T007、T010 |
| FR-2         | T004、T008、T010                |
| FR-3～FR-5    | T008、T011、T012                |
| FR-6～FR-8    | T011、T012、T013                |
| FR-9         | T014、T016                     |
| FR-10        | T008、T015                     |
| FR-11        | T008、T011、T015                |
| FR-12        | T009、T011、T014、T019           |
| SC-1～SC-8 汇总 | T010、T012、T013、T015、T016、T018 |

## 实施策略

首个可验收增量为公共基础加 US1，交付商户密钥分配、本地命令和可验证的签名通知。后续以 US2 补齐商户验签接入和拒绝场景，再以 US3 完成重复通知与重新投递验收。

代码构建及协议核对可在本地执行，商户实际业务处理结果通过已配置测试接收端核实。涉及环境依赖的验收保留真实执行状态，交付时说明所需环境与已核实结果。

共 19 项任务，US1 为 7 项、US2 为 3 项、US3 为 3 项，公共准备、数据基础及整体验证为 6 项。

## 本次执行状态

代码、迁移、本地分配命令及商户文档已实现，构建、静态检查、已有加解密测试和隔离 MySQL/HTTP 验证通过。当前完成 14 项，T010、T012、T013、T015、T016 的本地验证已执行，外部商户与真实支付链路部分等待测试环境，保留未完成标记。

详细执行结果及所需外部联调信息见 [quickstart.md](quickstart.md) 第 6 节。
