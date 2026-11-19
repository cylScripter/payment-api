# payment-api

基于 Gin、gRPC、Protobuf 和 GORM/MySQL 的 Go 支付服务，提供健康检查、小程序登录和微信 JSAPI 下单。Go 版本以 `go.mod` 为准，当前为 `1.26.0`。

## 目录结构

```text
api/                    Protobuf 定义及生成代码
cmd/server/             服务入口
internal/controllers/   HTTP 请求绑定与统一响应
internal/service/       HTTP、RPC 共用的业务逻辑
internal/service/pay/   通用下单流程与渠道扩展
internal/repo/          数据库操作
internal/models/        数据模型与数据库入口
internal/router/        HTTP 路由与中间件
internal/rpc/           gRPC 服务注册
internal/config/        配置加载与热更新
internal/initials/      基础组件初始化与关闭
internal/pkg/           微信、微信支付、GORM 等基础组件
configs/                运行配置
migrations/             数据库 SQL
third_party/            公共 proto 依赖
```

## 配置与启动

默认读取项目根目录下的 `configs/config.yaml`，填写数据库、小程序配置和数据加解密口令。支付商户配置保存在 `pay_merchant_channel` 表中。配置示例使用占位值。

```yaml
encrypt_key: "你的数据加解密口令"

rpc:
  addr: ":9090"

wechat:
  app_id: "你的小程序AppID"
  app_secret: "你的小程序AppSecret"

database:
  host: "127.0.0.1"
  port: 3306
  user: "你的数据库账号"
  password: "你的数据库密码"
  max_idle: 10
  max_conn: 100
```

- 数据库名为 `jia_ye_pay`，表结构见 `migrations/init.sql`。启动时会加载商户与渠道缓存，需准备好数据库和表。
- 从库通过 `replica_host`、`replica_port`、`replica_user`、`replica_password` 配置，省略时读写使用主库。
- 微信支付从商户渠道表读取配置并解密密钥。JSAPI 下单通过 `merchant_account` 选择商户，小程序登录和下单须使用对应的 AppID。
- 配置默认每秒检查一次，更新失败时继续使用当前可用配置。HTTP、RPC 监听地址在启动时读取。

在项目根目录启动。

```bash
go mod download
make run
```

HTTP 默认监听 `:8080`，gRPC 默认监听 `:9090`。可指定配置文件、HTTP 端口和 Gin 模式。

```bash
CONFIG_PATH=/path/to/config.yaml PORT=9000 GIN_MODE=release go run ./cmd/server
```

构建并运行。

```bash
make build
./bin/payment-api
```

## 接口

商户创建订单及支付成功业务回调见 [支付对接接口文档](docs/商户支付接口.md)。

HTTP 与 RPC 共用 service。HTTP 响应统一为 `code`、`message`、`data`，成功时 `code` 为 `0`，失败时 `data` 为 `{}`，HTTP 状态码根据错误类型映射。

| HTTP | RPC | 说明 |
| --- | --- | --- |
| `GET /health` | `payment.health.v1.HealthService/Check` | 健康检查 |
| `POST /v1/auth/wechat/login` | `payment.auth.v1.AuthService/WechatLogin` | 用小程序登录 code 换取 OpenID |
| `POST /v1/payments/jsapi` | `payment.payment.v1.PaymentService/CreateJSAPIOrder` | JSAPI 下单 |

### 健康检查

```bash
curl http://localhost:8080/health
```

```json
{"code":0,"message":"ok","data":{"status":"ok"}}
```

### 小程序登录

使用小程序 `wx.login` 获取的临时 code，每次调用使用新 code。

```bash
curl -X POST http://localhost:8080/v1/auth/wechat/login \
  -H 'Content-Type: application/json' \
  -d '{"code":"替换为小程序wx.login返回的code"}'
```

```json
{"code":0,"message":"ok","data":{"openid":"用户在该小程序下的OpenID"}}
```

### JSAPI 下单

调用方提供核定金额及对应 AppID 的 OpenID，`total` 单位为整数分。

```bash
curl -X POST http://localhost:8080/v1/payments/jsapi \
  -H 'Content-Type: application/json' \
  -d '{
    "merchant_account": "你的商户账号",
    "out_trade_no": "PAY202611110001",
    "description": "订单商品描述",
    "total": 1,
    "openid": "替换为该AppID下的用户OpenID"
  }'
```

- 可选字段 `attach` 最多 128 个字符，`time_expire` 为未来的 RFC3339 时间。
- 成功时 `data` 包含 `prepay_id`、`appId`、`timeStamp`、`nonceStr`、`package`、`signType`、`paySign`，小程序从中读取参数调用 `wx.requestPayment`。
- 同一笔业务重试时保持原 `out_trade_no` 和全部请求参数。网络异常或响应验签失败时，先通过原商户订单号核实结果。

完整字段见 `api/auth/v1/auth.proto` 和 `api/payment/v1/payment.proto`。

## 接口开发

1. 在 `api/` 修改 proto，执行 `make proto` 并同步生成的 `*.pb.go`。
2. 在 `internal/service/` 实现业务逻辑，数据库操作放在 `internal/repo/`。
3. HTTP 接口在 `internal/controllers/` 绑定请求，通过 `ctx.Request.Context()` 调用 service，使用 `Controller.Response` 输出响应，并在 `internal/router/router.go` 显式注册路由。
4. 新增 RPC 服务在 `internal/rpc/server.go` 注册。

首次生成代码需安装 `protoc` 3.20 或更高版本，并安装项目指定的 Go 插件。

```bash
make tools
make proto
```

配置通过 `config.Get()` 获取，基础组件统一由 `initials.Init()` 初始化、`initials.Close()` 关闭。切换配置来源可实现 `config.Source`，通过 `initials.Init(source)` 或 `config.SetSource(source)` 接入。

通用下单流程位于 `internal/service/pay/`，通过 `Payer` 和 `ChannelRegistry` 扩展渠道，商户加载、幂等检查、持久化和渠道接入仍有待完善。流程说明见该目录的 README。
