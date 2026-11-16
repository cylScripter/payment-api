<!--
Sync Impact Report
- 版本由 2.0.0 更新为 2.1.0，新增数据库访问原则并实质扩展 Go 实现约定，递增次版本。
- 扩展原则 III. 简洁的 Go 实现，补充先设计后实现、职责划分、辅助函数、参数校验及命名要求。
- 新增原则 IV. 数据模型与数据库访问，明确 GORM 映射、MySQLXxx 对象及私有查询条件的构造方式。
- 移除章节为空。
- 待办与延后事项均为空。
本报告用于审阅本次修订，提交宪章前移除此注释。
-->

# payment-api Constitution

## Core Principles

### I. 模块职责与分层

- HTTP 请求接收、参数绑定及应答处理放在 controller，业务逻辑放在 service，
  数据库操作放在 repo，模型及数据库入口放在 models。
- HTTP 与 RPC 必须复用 service。HTTP 路由在 router 显式注册，RPC 服务在 rpc 注册。
- 渠道 SDK 封装、回调解析器和公共组件放在 pkg；组件初始化与关闭统一接入 initials。
  配置加载及热更新由 config 管理。
- 文件归属必须遵循下方项目结构，使业务流程、协议接入和基础组件具有明确维护位置。

### II. 接口契约与兼容性

- 接口以 `.proto` 为源，修改后执行 `make proto` 同步生成代码。service 优先复用
  已有的 proto 请求和响应类型。
- 必须保持已有字段编号、JSON 字段名和调用行为的兼容性。需求涉及契约调整时，
  必须说明对现有调用方的影响。
- 普通 HTTP 接口沿用 `code`、`message`、`data` 响应格式及 `protojson` 序列化。
  支付渠道回调必须按对应渠道的应答协议返回。
- service 错误沿用 gRPC `codes` 和 `status`；数据库及外部请求必须传递调用方的
  `context.Context`，HTTP 入口使用 `ctx.Request.Context()`。

### III. 简洁的 Go 实现

- 实现必须围绕当前需求，优先复用现有逻辑、类型、依赖和调用方式，遵循所在模块的代码风格。
- 实现代码前必须先设计，明确结构体职责、对象协作和调用流程，采用适合当前需求的设计模式。
  按职责组织方法，避免单个函数堆积大量业务逻辑。
- 函数和方法的参数一般不超过 3 个。复杂参数构造采用项目已有的 option 模式。
- 根据实际职责抽象接口、面向结构体编程，保持设计与当前需求匹配，避免过度设计。
  接口、抽象和辅助函数必须具有当前明确用途，减少接口断言。
- 少定义辅助函数，避免机械拆分 `normalizePage`、`encodeCursor`、`decodeCursor`、`postDTO` 等函数。
- 减少不必要的参数校验，按用户约定省略 `req == nil` 判断，保持已有显式忽略 error 的写法。
- 命名必须简短且能准确表达语意，沿用所在模块的惯用名称。
- 修改过的 Go 文件必须使用 `gofmt`。结构体初始化时每个字段单独占一行。

### IV. 数据模型与数据库访问

- 数据模型字段必须添加明确的 `gorm` 标签，表模型必须提供 `TableName()` 方法。
- 数据库访问使用持有私有字段 `db *gorm.DB` 的 `MySQLXxx` 对象，
  通过 `NewMySQLXxx(db)` 构造。
- 查询条件必须使用 `ReqXxx` 结构体，字段保持私有，通过 Functional Options 构造。
- 查询请求构造器签名为 `NewReqXxx(opts ...func(*ReqXxx)) ReqXxx`。
- 单字段选项遵循 `WithReqXxxBrandCode(brandCode string) func(*ReqXxx)` 的形式。
- 分页选项遵循 `WithReqXxxPage(page, size int32) func(*ReqXxx)` 的形式。

## 项目结构

项目模块名为 `payment-api`，Go 版本以 `go.mod` 为准。HTTP 使用 Gin，RPC 使用 gRPC，
接口由 Protobuf 定义，数据库访问使用 GORM 和 MySQL。

```text
/Users/chenyinglin/project/payment-api/
├── .agents/skills/          # Spec Kit 技能
├── .specify/                # Spec Kit 规范、模板和工作流
├── api/                     # Protobuf 接口定义及生成代码
│   ├── auth/v1/             # 认证接口
│   ├── health/v1/           # 健康检查接口
│   └── payment/v1/          # 支付接口
├── bin/                     # 构建产物及代码生成工具
├── cmd/server/
│   └── main.go              # 服务启动入口
├── configs/                 # 运行配置和渠道配置
├── internal/
│   ├── config/              # 配置加载、配置来源及热更新
│   ├── controllers/         # HTTP 请求接收及响应处理
│   ├── initials/            # 基础组件初始化与关闭
│   ├── models/              # 数据模型及数据库入口
│   ├── pkg/                 # 公共组件
│   │   ├── gormhelper/      # 数据库连接与连接池管理
│   │   ├── gormlogger/      # GORM 日志
│   │   ├── helpers/         # 加解密、金额转换等工具
│   │   ├── idgen/           # 分布式订单号生成
│   │   ├── pay/             # 支付基础组件
│   │   │   └── wechatpay/   # 微信支付客户端及回调解析器
│   │   ├── wechat/          # 微信基础客户端
│   │   └── payer.go         # 支付接口定义
│   ├── repo/                # 数据库访问
│   ├── router/              # HTTP 路由注册
│   ├── rpc/                 # gRPC 服务注册
│   └── service/             # HTTP 与 RPC 共用业务逻辑
│       ├── merchant/        # 商户及渠道配置缓存
│       ├── notify/          # 支付回调处理及业务通知
│       └── pay/             # 下单流程、渠道注册及渠道实现
├── migrations/              # 数据库初始化及增量 SQL
├── third_party/             # 公共 Protobuf 依赖
├── AGENTS.md                # 项目协作约定
├── CLAUDE.md                # Claude 项目说明
├── Makefile                 # 代码生成、构建和运行命令
├── go.mod                   # Go 模块与依赖声明
├── go.sum                   # 依赖校验信息
└── README.md                # 项目说明
```

## 开发流程与验证

1. 开始修改前查看 `git status`、相关模块和同类实现，保留用户已有修改，确认本次需求范围。
2. 修改代码符号前按项目协作约定完成影响分析。新增接口同步处理 proto、controller、service
   及相应路由或 RPC 注册，数据库变更同步模型和迁移 SQL。
3. 按改动范围执行 `go build ./...`、`go vet ./...`，按需运行现有测试。
   无特殊需求时不新增 `xxx_test.go`。文档改动核对目录、命令和实际代码。
4. 交付时说明实际改动、所采用的必要假设、验证结果和仍待完成或联调的事项。
   提交前按项目协作约定检查变更范围。

## Governance

本宪章记录项目的结构与开发原则，具体操作约定见
[项目协作约定](AGENTS.md)。
用户当前明确要求优先于既有约定，调整时必须说明对应影响。

修订必须记录修改原因、影响范围，并同步更新版本号和修订日期。
原则删除或不兼容重定义递增主版本；新增原则、章节或实质扩展递增次版本；
文字澄清与非语义修正递增修订版本。首次制定版本为 `1.0.0`。

需求说明、方案和交付审阅必须核对相关原则。出现偏离时，必须记录对应的明确需求及原因，
供后续维护和审阅核实。

**Version**: 2.1.0 | **Ratified**: 2026-11-16 | **Last Amended**: 2026-11-16
