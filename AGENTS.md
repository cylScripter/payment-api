# 项目协作约定

## 沟通与工作方式

- 默认使用中文沟通，按 Go 后端工程的习惯编写代码和说明。
- 当前项目为 `payment-api`，项目根路径为 `/Users/chenyinglin/project/payment-api`。
- 开始修改前查看相关代码和 `git status`，保留用户已有的修改。
- 围绕当前需求做最小必要改动，优先复用现有逻辑、类型和依赖。
- 完成后说明实际改动和验证结果，有未完成或未验证的内容时明确指出。

## 项目结构

- Go module 为 `payment-api`，Go 版本以 `go.mod` 为准，当前为 `1.26.0`。
- HTTP 使用 Gin，RPC 使用 gRPC，接口及请求响应由 Protobuf 定义，数据库使用 GORM 和 MySQL。
- `cmd/server/main.go` 为启动入口，负责 HTTP、RPC 监听和优雅退出。
- `api/<业务>/v1/` 保存 `.proto` 定义和生成的 Go 代码。
- `internal/controllers/` 负责 HTTP 请求绑定、调用 service 和输出响应。
- `internal/service/` 实现业务逻辑，HTTP 与 RPC 复用同一套 service。
- `internal/service/pay/` 保存通用下单流程、渠道注册和 `Payer` 接口。
- `internal/service/merchant/` 保存商户与渠道缓存。
- `internal/repo/` 负责数据库操作，`internal/models/` 保存数据模型和数据库入口。
- `internal/pkg/` 保存微信客户端、微信支付客户端和 GORM 等基础组件。
- `internal/config/` 负责配置加载及热更新，`internal/initials/` 统一初始化和关闭基础组件。
- `internal/router/router.go` 显式注册 HTTP 路由，`internal/rpc/server.go` 注册 RPC 服务。
- `migrations/` 保存数据库 SQL，`configs/` 保存运行配置，`third_party/` 保存公共 proto 依赖。

## Go 编码约定

- 编码前先阅读当前模块及同类功能的现有实现，遵循项目已有的代码风格、命名、结构、调用方式和错误处理习惯，优先沿用现有写法。未经用户要求，不按个人偏好引入新的实现风格或重构现有代码。
- 代码尽量采用简单实现，减少不必要的抽象和复杂逻辑。
- 减少不必要的安全性校验。
- 用户已经忽略的 error 处理，保持现有写法，不主动补充错误处理。
- 没有特殊需求不要新增 `xxx_test.go`。
- 函数和方法的参数一般不超过 3 个，复杂参数构造可以使用 option 模式，沿用 `opts ...func(*T)` 和 `WithXxx` 的写法。
- 尽量少定义缺乏实际复用价值的辅助函数，例如 `normalizePage`、`encodeCursor`、`decodeCursor`、`postDTO`。
- 不用判断 `req == nil`。
- 命名简洁、明确，沿用所在包的风格，使用 `gofmt` 格式化修改过的 Go 文件。
- controller 保持请求绑定和响应处理职责，业务逻辑放在 service，数据库操作放在 repo。
- service 优先使用已有的 proto 请求和响应类型。
- 数据库及外部请求沿用调用方的 `context.Context`，HTTP 入口通过 `ctx.Request.Context()` 传入。
- 沿用现有错误处理方式，service 对外错误使用 gRPC `codes` 和 `status`，HTTP 通过 `Controller.Response` 统一映射。

## 接口修改

- 接口定义以 `.proto` 为源，修改后执行 `make proto`，同步生成的 `*.pb.go`，生成代码通过生成命令更新。
- 保持已有字段编号、JSON 字段名和调用行为的兼容性，按需求调整契约。
- 新增 HTTP 接口时同步添加 controller 和显式路由。`google.api.http` 注解用于描述接口，HTTP 路由仍需手动注册。
- 新增 RPC 服务时在 `internal/rpc/server.go` 注册，沿用 HTTP 与 RPC 共用 service 的结构。
- HTTP 响应沿用 `code`、`message`、`data` 格式，序列化沿用 `protojson`。

## 支付与基础组件

- 修改通用下单流程前查看 `internal/service/pay/` 的实际实现及其 README。商户加载、幂等检查、持久化和渠道接入按当前代码确认完成情况。
- 扩展支付渠道时沿用 `Payer`、`ChannelRegistry` 和现有下单流程，每次请求创建独立的下单状态，SDK 客户端可共享。
- 金额单位以具体 proto 字段约定为准，当前通用下单的 `amount` 为元，JSAPI 下单的 `total` 为整数分，转换时核对精度和单位。
- JSAPI 下单重试沿用原商户订单号和原请求参数，出现结果不确定的情况时沿用现有结果核实逻辑。
- 配置通过 `config.Get()` 获取，默认读取 `configs/config.yaml`，可通过 `CONFIG_PATH` 指定配置文件。
- 新增基础组件接入 `initials` 的生命周期，修改热更新时保留配置校验失败继续使用当前可用配置的行为。
- 数据库操作复用现有 `models` 和 `gormhelper` 入口，SQL 与模型修改保持一致。
- 配置示例和日志中的密码、AppSecret、私钥等敏感信息使用占位或脱敏内容。

## 常用命令与验证

```bash
go mod download
make tools
make proto
make build
make run
```

- `make tools` 安装 Makefile 指定版本的 protobuf Go 插件到 `bin/`，`make proto` 依赖本机已安装的 `protoc`。
- 修改 Go 代码后按影响范围执行构建，常用 `go build ./...` 和 `go vet ./...`。按需运行现有测试。
- 仅修改文档时核对内容与实际代码、命令是否一致。
- 运行服务会初始化数据库、商户缓存和微信相关客户端，联调前确认使用的配置及依赖环境。
- HTTP 默认端口为 `8080`，可通过 `PORT` 修改。RPC 地址来自 `rpc.addr`，默认 `:9090`。健康接口为 `GET /health`。

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **payment-api** (830 symbols, 1561 relationships, 53 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource                                     | Use for                                  |
|----------------------------------------------|------------------------------------------|
| `gitnexus://repo/payment-api/context`        | Codebase overview, check index freshness |
| `gitnexus://repo/payment-api/clusters`       | All functional areas                     |
| `gitnexus://repo/payment-api/processes`      | All execution flows                      |
| `gitnexus://repo/payment-api/process/{name}` | Step-by-step execution trace             |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
