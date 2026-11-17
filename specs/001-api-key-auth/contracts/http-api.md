# HTTP 契约 — 商户 API_KEY

**规格** [spec.md](../spec.md) | **数据模型** [data-model.md](../data-model.md)

## 路由与凭证

| 方法与路径                                                    | 授权要求                        |
|----------------------------------------------------------|-----------------------------|
| POST /v1/auth/api-keys                                   | 平台管理凭证                      |
| POST /v1/auth/wechat/login                               | 有效商户 API_KEY                |
| POST /v1/payments/jsapi                                  | 有效商户 API_KEY，且请求商户账号与密钥归属一致 |
| GET /health                                              | API_KEY 豁免                  |
| POST /payment-api/v1/wechat/pay/notify/:merchant_account | API_KEY 豁免，沿用渠道回调处理         |
| 加入受保护分组的后续 HTTP 业务路由                                         | 默认使用商户 API_KEY              |

凭证通过唯一的 `Authorization: Bearer <凭证值>` 传入。Bearer 大小写不敏感，密钥大小写敏感。缺失、空值、其他认证方案、多个 Authorization 值、拼接多个凭证及带空白的密钥均返回 401。查询参数及请求体中的密钥不能满足认证要求。

router 使用 Gin 路由分组挂载中间件，公开组直接处理健康检查和支付回调，商户与支付组挂载 Auth，管理组挂载 AdminAuth。后续接口按权限加入对应分组。未匹配路径沿用现有 404。现有 gRPC 业务接口的商户 API_KEY 鉴权属于规格排除范围，新增签发方法遵循下文的管理凭证契约。

## 生成商户 API_KEY

`POST /v1/auth/api-keys`，请求头包含 `Authorization: Bearer ${ADMIN_KEY}` 和 `Content-Type: application/json`。

请求由 `payment.auth.v1.GenerateAPIKeyReq` 定义。

| 字段               | 编号 | JSON 名           | 类型     | 约束                            |
|------------------|----|------------------|--------|-------------------------------|
| merchant_account | 1  | merchant_account | string | 必填，非纯空白，最长 128 字符，与既有商户账号精确匹配 |

成功响应 HTTP 200，添加 `Cache-Control: no-store`。data 由 `GenerateAPIKeyResp` 定义。

| 字段 | 编号 | JSON 名 | 类型 | 含义 |
| --- | --- | --- | --- | --- |
| api_key | 1 | api_key | string | 新签发的 35 字符（sk- 加 32 位小写十六进制）完整密钥 |
| merchant_account | 2 | merchant_account | string | 所属商户账号 |

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "api_key": "sk-ead347eaa7ea43a2ba691adf0422c264",
    "merchant_account": "merchant-a"
  }
}
```

在现有 `api/auth/v1/auth.proto` 中扩展 `AuthService.GenerateAPIKey`，通过 `google.api.http` 注解描述此路由。请求响应消息定义在同一文件中，JSON 名通过字段选项明确指定，HTTP 路由手工注册。

### AuthService 签发方法

现有服务注册提供 `/payment.auth.v1.AuthService/GenerateAPIKey`，复用相同的请求响应消息及签发逻辑。调用方使用 `authorization: Bearer <管理凭证>` incoming metadata。

HTTP 的 AdminAuth 直接校验请求头中的 Authorization，RPC 入口从 incoming metadata 读取该值，均通过 middleware.AuthorizeAdmin(ctx, values) 完成管理授权后再调用 Auth.GenerateAPIKey 处理业务。RPC 缺失或未知凭证返回 `Unauthenticated`，商户 API_KEY 返回 `PermissionDenied`，正确管理凭证按相同商户校验和事务规则签发。数据库及配置错误沿用下表的 gRPC code。

### 签发错误

| 情况                   | HTTP | code               |
|----------------------|------|--------------------|
| 管理凭证缺失、未知或格式错误       | 401  | 16 Unauthenticated |
| 已签发商户 API_KEY 调用管理接口 | 403  | 7 PermissionDenied |
| 管理凭证为空配置             | 503  | 14 Unavailable     |
| 管理认证通过，账号为空、超长或请求体无效 | 400  | 3 InvalidArgument  |
| 签发失败，包括商户不可用或数据库操作失败 | 503 | 14 Unavailable |

处理顺序为凭证格式检查、读取管理配置、比较管理凭证、签发参数校验及事务。管理凭证不匹配时查询密钥摘要，已签发商户密钥返回 403，其余返回 401，查询错误返回 503。空管理配置对格式正确的签发请求返回 503。

每次成功生成均追加密钥。失败响应沿用统一格式，data 为 `{}`。提交结果不确定或响应丢失时，调用方可以重新签发。

## 商户业务鉴权

认证使用摘要关联当前商户 ID、账号及状态。未知密钥、商户不存在或禁用返回 HTTP 401、code 16，主库查询故障返回 HTTP 503、code 14。支付请求指定账号时，商户缓存 ID 与认证商户 ID 不一致同样返回 HTTP 503、code 14，以等待商户配置同步。

中间件通过通用 JSON 参数读取商户账号，对非空 `merchant_account` 与密钥所属商户账号精确比较，不一致返回 HTTP 403、code 7。protobuf JSON 别名 `merchantAccount` 遵循相同校验，商户号缺失、空值、类型错误或同时提供两种命名时返回 400，其他业务字段由 controller 绑定。

```json
{
  "code": 7,
  "message": "无权访问该商户",
  "data": {}
}
```

权限校验通过后，中间件恢复原始请求体并放行，商户账号随原有请求参数进入业务处理。没有商户账号字段的微信登录请求，经中间件校验密钥及所属商户状态后进入登录流程。后续含商户字段的 HTTP 接口，加入相应受保护分组并配置其参数校验。

认证失败须在业务调用前返回。现有业务成功及错误结果沿用原有契约，context 取消和超时沿用统一映射。普通业务响应、错误及日志只输出必要的固定错误信息与商户标识。
