# 联调验证 — 商户 API_KEY

本指南在功能实现后执行。接口及错误码见 [HTTP 契约](contracts/http-api.md)，表结构及一致性规则见 [数据模型](data-model.md)。

## 准备

使用可联调的测试环境，准备两个正常商户 A、B，以及一个禁用商户。确认所用商户账号各自只有一条记录。现有数据库和微信组件须能正常初始化。

在运行配置中增加管理凭证，并按环境注入实际值。

```yaml
auth:
  admin_key: "<platform-admin-key>"
```

实施后应用 `migrations/20261117_pay_merchant_api_key.sql`，核对初始化 SQL 中对应表定义。迁移由环境现有发布流程执行。

```bash
make proto
go build ./...
go vet ./...
CONFIG_PATH=/path/to/test-config.yaml make run
```

命令在项目根目录执行。验证命令使用 Bash、curl 和 jq。以下环境变量按测试环境填写，管理凭证输入时隐藏。

```bash
export PAY_AUTH_BASE=http://127.0.0.1:8080
export PAY_AUTH_MERCHANT_A=merchant-a
export PAY_AUTH_MERCHANT_B=merchant-b
read -r -s -p '管理凭证 ' PAY_AUTH_ADMIN_KEY
printf '\n'
```

## 1. 首次签发和管理权限

```bash
PAY_AUTH_KEY_A="$(
  jq -n --arg account "$PAY_AUTH_MERCHANT_A" '{merchant_account: $account}' |
  curl --fail-with-body -sS "$PAY_AUTH_BASE/v1/auth/api-keys" \
    -H "Authorization: Bearer $PAY_AUTH_ADMIN_KEY" \
    -H 'Content-Type: application/json' --data-binary @- |
  jq -er 'select(.code == 0) | .data.api_key'
)"
test "${#PAY_AUTH_KEY_A}" -eq 43
```

预期拿到密钥且长度检查通过。使用相同方式为 B 签发并保存为 `PAY_AUTH_KEY_B`。再次为 A 签发，新旧密钥必须不同且均可通过认证。

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  "$PAY_AUTH_BASE/v1/auth/api-keys" \
  -H "Authorization: Bearer $PAY_AUTH_KEY_A" \
  -H 'Content-Type: application/json' \
  --data '{"merchant_account":"merchant-b"}'
```

预期 403。去掉 Authorization 或改为未知凭证时预期 401。使用管理凭证指定不存在商户时预期 404，禁用商户预期 403，空账号预期 400。每种拒绝场景均核对密钥记录数量无增加。

## 2. 商户身份及归属

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  "$PAY_AUTH_BASE/v1/auth/wechat/login" \
  -H "Authorization: Bearer $PAY_AUTH_KEY_A" \
  -H 'Content-Type: application/json' --data '{"code":""}'
```

预期 400，来自原有登录参数校验，证明请求已通过商户认证。去掉凭证或改为未知密钥后，同一请求预期 401。

```bash
jq -n --arg account "$PAY_AUTH_MERCHANT_B" '{merchant_account: $account}' |
curl -sS -o /dev/null -w '%{http_code}\n' \
  "$PAY_AUTH_BASE/v1/payments/jsapi" \
  -H "Authorization: Bearer $PAY_AUTH_KEY_A" \
  -H 'Content-Type: application/json' --data-binary @-
```

预期 403，订单及其他业务记录增量为 0。把 JSON 字段改成 `merchantAccount` 重复请求，仍预期 403。联调时确认中间件在调用业务 handler 前拒绝跨商户请求。

准备商户 A 的有效测试下单请求文件，以 A 的密钥发起完整请求，预期返回原有下单成功结果。核对中间件读取并恢复请求体后，controller 可完整绑定请求，商户账号及其余字段按请求参数进入下单流程。

```bash
curl --fail-with-body -sS "$PAY_AUTH_BASE/v1/payments/jsapi" \
  -H "Authorization: Bearer $PAY_AUTH_KEY_A" \
  -H 'Content-Type: application/json' \
  --data-binary @/path/to/merchant-a-order.json
```

## 3. 异常凭证

对登录请求分别使用缺失凭证、仅 `Bearer`、Basic 方案、未知密钥、重复 Authorization 头、同头拼接两个密钥，全部预期 401。将凭证仅放入查询串或请求体时同样预期 401。正常请求的 Bearer 方案改成小写后仍可通过认证。

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  "$PAY_AUTH_BASE/v1/auth/wechat/login" \
  -H "Authorization: Bearer $PAY_AUTH_KEY_A" \
  -H "Authorization: Bearer $PAY_AUTH_KEY_B" \
  -H 'Content-Type: application/json' --data '{"code":""}'
```

预期 401，业务写入增量为 0。

## 4. 持久化、并发及故障

- 连续为商户签发 100 把密钥并检查互异性。逐一调用空 code 登录请求，均预期原业务校验返回 400。保留首把密钥再检查一次。
- 使用并发签发请求重复验证，成功返回的每把密钥均可用且商户归属正确。
- 重启服务，以首把密钥再次调用，仍预期 400。将 `PAY_AUTH_BASE` 指向同一部署的另一实例，立即重复验证。多实例共用同一主库，单机启动第二实例时需使用不同 HTTP 端口及 `rpc.addr`。
- 测试环境中通过既有商户管理操作禁用 A，下一次认证预期 401。恢复正常后重试，按同一商户 ID 继续校验。
- 在可控测试环境让主库临时不可用，格式正确的业务凭证请求预期 503，使用正确管理凭证的签发请求预期 503，业务写入增量为 0。恢复后再次验证已有密钥。
- 若构造签发响应丢失场景，重新签发并验证返回的新密钥可用，已成功交付的旧密钥仍可用。

## 5. 豁免与配置热更新

```bash
curl --fail-with-body -sS "$PAY_AUTH_BASE/health"
```

预期现有健康检查成功结果。使用环境既有的渠道回调样本，在携带渠道校验材料且没有 API_KEY 的情况下验证回调按原规则处理。无效渠道回调按现有规则处理。

更新运行配置中的管理密钥，等待配置监听发布新快照。新凭证可签发，旧凭证返回 401，已有商户密钥继续可用。将管理字段改成错误 YAML 类型或含空白的非空值，预期记录字段校验错误并继续使用上一份可用快照。将管理密钥明确设置为空后，签发请求预期 503。

核对 gRPC 现有服务调用及响应，验证其沿用原有调用契约。

现有 AuthService 中新增的签发方法还需验证管理授权。以下命令使用 grpcurl，并显式加载本地 proto。

```bash
export PAY_AUTH_RPC=127.0.0.1:9090
jq -n --arg account "$PAY_AUTH_MERCHANT_A" '{merchant_account: $account}' |
grpcurl -plaintext -import-path . -import-path third_party \
  -proto api/auth/v1/auth.proto -d @ "$PAY_AUTH_RPC" \
  payment.auth.v1.AuthService/GenerateAPIKey
```

预期 `Unauthenticated`。添加 `-H "authorization: Bearer $PAY_AUTH_KEY_A"` 时预期 `PermissionDenied`。使用管理凭证发起相同请求，预期签发成功。

```bash
jq -n --arg account "$PAY_AUTH_MERCHANT_A" '{merchant_account: $account}' |
grpcurl -plaintext -import-path . -import-path third_party \
  -proto api/auth/v1/auth.proto \
  -H "authorization: Bearer $PAY_AUTH_ADMIN_KEY" \
  -d @ "$PAY_AUTH_RPC" payment.auth.v1.AuthService/GenerateAPIKey |
jq -e --arg account "$PAY_AUTH_MERCHANT_A" \
  '.merchant_account == $account and (.api_key | length) == 43'
```

预期输出 `true`。拒绝场景的密钥记录增量均为 0。

## 6. 日志和存储核对

以原文密钥匹配测试日志，匹配次数预期为 0。数据库仅有 64 字符摘要和商户 ID 等元数据。错误响应及普通业务响应均无完整密钥，签发响应有 `Cache-Control: no-store`。使用商户 A 的密钥访问 B 的拒绝场景，核对订单记录增量为 0。

执行完毕后逐项核对规格 SC-1 至 SC-9，并记录环境、响应状态、商户归属和业务写入结果。
