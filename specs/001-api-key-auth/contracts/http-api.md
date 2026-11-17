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
| 后续已注册的 HTTP 业务路由                                         | 默认使用商户 API_KEY              |

凭证通过唯一的 `Authorization: Bearer <凭证值>` 传入。Bearer 大小写不敏感，密钥大小写敏感。缺失、空值、其他认证方案、多个 Authorization 值、拼接多个凭证及带空白的密钥均返回 401。查询参数及请求体中的密钥不能满足认证要求。

豁免使用 HTTP 方法和匹配后的路由模板精确判断。未匹配路径沿用现有 404。现有 gRPC 业务接口的商户 API_KEY 鉴权属于规格排除范围，新增签发方法遵循下文的管理凭证契约。

## 生成商户 API_KEY

`POST /v1/auth/api-keys`，请求头包含 `Authorization: Bearer ${ADMIN_KEY}` 和 `Content-Type: application/json`。

请求由 `payment.auth.v1.GenerateAPIKeyReq` 定义。

| 字段               | 编号 | JSON 名           | 类型     | 约束                            |
|------------------|----|------------------|--------|-------------------------------|
| merchant_account | 1  | merchant_account | string | 必填，非纯空白，最长 128 字符，与既有商户账号精确匹配 |

成功响应 HTTP 200，添加 `Cache-Control: no-store`。data 由 `GenerateAPIKeyResp` 定义。

| 字段 | 编号 | JSON 名 | 类型 | 含义 |
| --- | --- | --- | --- | --- |
| api_key | 1 | api_key | string | 新签发的 43 字符完整密钥 |
| merchant_account | 2 | merchant_account | string | 所属商户账号 |

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "api_key": "<generated-api-key>",
    "merchant_account": "merchant-a"
  }
}
```

在现有 `api/auth/v1/auth.proto` 中扩展 `AuthService.GenerateAPIKey`，通过 `google.api.http` 注解描述此路由。请求响应消息定义在同一文件中，JSON 名通过字段选项明确指定，HTTP 路由手工注册。

### AuthService 签发方法

现有服务注册提供 `/payment.auth.v1.AuthService/GenerateAPIKey`，复用相同的请求响应消息及签发逻辑。调用方使用 `authorization: Bearer <管理凭证>` incoming metadata。

HTTP 中间件将原始 Authorization 值放入请求 context 的 incoming metadata，controller 沿用该 context 调用 `Auth.GenerateAPIKey`。该方法在签发前执行管理授权检查。RPC 缺失或未知凭证返回 `Unauthenticated`，商户 API_KEY 返回 `PermissionDenied`，正确管理凭证按相同商户校验和事务规则签发。数据库及配置错误沿用下表的 gRPC code。

### 签发错误

| 情况                   | HTTP | code               |
|----------------------|------|--------------------|
| 管理凭证缺失、未知或格式错误       | 401  | 16 Unauthenticated |
| 已签发商户 API_KEY 调用管理接口 | 403  | 7 PermissionDenied |
| 管理凭证为空配置             | 503  | 14 Unavailable     |
| 管理认证通过，账号为空、超长或请求体无效 | 400  | 3 InvalidArgument  |
| 商户不存在                | 404  | 5 NotFound         |
| 商户禁用                 | 403  | 7 PermissionDenied |
| 同账号多个商户，或生成摘要发生唯一冲突  | 409  | 6 AlreadyExists    |
| 数据库连接、查询或提交失败        | 503  | 14 Unavailable     |

处理顺序为凭证格式检查、读取管理配置、比较管理凭证、签发参数校验及事务。管理凭证不匹配时查询密钥摘要，已签发商户密钥返回 403，其余返回 401，查询错误返回 503。空管理配置对格式正确的签发请求返回 503。

每次成功生成均追加密钥。失败响应沿用统一格式，data 为 `{}`。提交结果不确定或响应丢失时，调用方可以重新签发。

## 商户业务鉴权

认证使用摘要关联当前商户 ID、账号及状态。未知密钥、商户不存在或禁用返回 HTTP 401、code 16，主库查询故障返回 HTTP 503、code 14。

下单请求由中间件使用现有 `CreateOrderReq` 及相同 protojson 选项解析，对非空 `merchant_account` 与密钥所属商户账号精确比较，不一致返回 HTTP 403、code 7。protobuf JSON 别名 `merchantAccount` 遵循相同校验，重复或冲突字段遵循原有 protojson 错误规则。账号缺失时沿用原业务契约。

```json
{
  "code": 7,
  "message": "无权访问该商户",
  "data": {}
}
```

权限校验通过后，中间件恢复原始请求体并放行，商户账号随原有请求参数进入业务处理。没有商户账号字段的微信登录请求，经中间件校验密钥及所属商户状态后进入登录流程。后续含商户字段的 HTTP 接口，在中间件增加对应 proto 请求类型的参数校验。

认证失败须在业务调用前返回。现有业务成功及错误结果沿用原有契约，context 取消和超时沿用统一映射。普通业务响应、错误及日志只输出必要的固定错误信息与商户标识。
