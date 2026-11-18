# 商户支付对接接口文档

本文说明商户创建订单和接收支付成功回调的对接方式。创建订单支持微信、支付宝、抖音支付，业务回调目前支持微信支付。

## 接入准备

接入前由支付服务提供对应环境的服务地址、商户账号 `account` 和 API Key，并开通商户需要使用的支付渠道及应用。

商户同时取得独立的回调密钥 `notify_secret`，保存到商户后端，用于验证支付成功通知。API Key 用于商户请求支付服务，`notify_secret` 用于商户验证回调，两者分别管理。验签时根据约定的商户账号使用对应回调密钥。

商户提供支付成功回调地址，由支付服务完成配置。示例服务地址使用 `https://pay.example.com`，商户回调地址使用 `https://merchant.example.com/payment/notify`。

基本流程如下。

1. 业务后端携带商户 API Key 调用创建订单接口。
2. 保存响应中的支付单号 `data.outTradeNo`，关联业务订单号 `order_no`。
3. 将 `data.payParams` 交给对应客户端调起支付。
4. 支付成功后，支付服务向商户提供的回调地址发送支付结果。
5. 业务后端校验时间戳和签名，核对订单号及金额，幂等处理业务后返回 HTTP 200 或 204。

## 创建订单

### 请求地址

```http
POST /merchant-api/v1/create/order
Content-Type: application/json
Authorization: Bearer <API_KEY>
```

API Key 所属商户必须与请求中的 `account` 一致。API Key 由业务后端保存并用于请求支付服务。

### 请求参数

| 字段                | 类型     | 必填   | 说明                                                     |
|-------------------|--------|------|--------------------------------------------------------|
| `account`         | string | 是    | 支付服务分配的商户账号                                            |
| `order_no`        | string | 是    | 业务方订单号，用于关联业务订单                                        |
| `description`     | string | 是    | 订单商品描述，长度限制见下表                                         |
| `amount`          | number | 是    | 订单金额，单位为元，大于 0，最多两位小数，例如 `0.01`、`12.50`                |
| `trade_type`      | string | 是    | 支付方式，支持 `JSAPI`、`NATIVE`、`H5`、`APP`，按大小写传入             |
| `channel`         | string | 否    | 支付渠道，支持 `wechat`、`alipay`、`douyin`，省略或空字符串时使用 `wechat` |
| `openid`          | string | 条件必填 | `JSAPI` 必填，使用对应渠道、对应应用下的用户标识。支付宝填写 `buyer_open_id`     |
| `attach`          | string | 否    | 渠道附加数据，长度限制见下表                                         |
| `payer_client_ip` | string | 条件必填 | 微信、抖音 `H5` 支付必填，填写用户终端有效 IP 地址                         |
| `h5_type`         | string | 条件必填 | 微信、抖音 `H5` 支付必填，取值见下表                                  |

| 渠道           | 参数约束                                                                                                                                                                                                        |
|--------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 微信 `wechat`  | `order_no` 为 6～32 位数字、字母或 `_`、`-`、`\|`、`*`。`description` 最多 127 个字符，`attach` 最多 128 个字符。`description`、`attach`、`openid` 使用 UTF-8 一至三个字节编码的字符。JSAPI 的 `openid` 最多 128 个字符。`h5_type` 支持 `Wap`、`iOS`、`Android` |
| 抖音 `douyin`  | `description` 最多 127 个字符，`attach` 最多 1024 个字符。JSAPI 的 `openid` 最多 128 个字符。`h5_type` 支持 `Wap`、`iOS`、`Android`、`HarmonyOS`                                                                                    |
| 支付宝 `alipay` | `description` 最多 256 个字符，`amount` 最大为 100000000 元，`attach` 经 URL 编码后最多 512 个字符                                                                                                                              |

JSAPI 的 `openid` 需与开通支付时使用的应用 AppID 对应。

### 微信 JSAPI 请求示例

```bash
curl --request POST 'https://pay.example.com/merchant-api/v1/create/order' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer <API_KEY>' \
  --data '{
    "account": "merchant_demo",
    "order_no": "ORDER202611180001",
    "description": "商品订单支付",
    "amount": 12.50,
    "openid": "用户在对应AppID下的OpenID",
    "attach": "业务附加信息",
    "trade_type": "JSAPI",
    "channel": "wechat"
  }'
```

### 微信 H5 请求示例

```json
{
  "account": "merchant_demo",
  "order_no": "ORDER202611180002",
  "description": "商品订单支付",
  "amount": 12.50,
  "trade_type": "H5",
  "channel": "wechat",
  "payer_client_ip": "203.0.113.10",
  "h5_type": "Wap"
}
```

### 成功响应

HTTP 状态码为 `200`，`code` 为 `0`。

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "outTradeNo": "123456789012345678",
    "tradeType": "JSAPI",
    "payParams": {
      "prepay_id": "wx_example_prepay_id",
      "appId": "wx_example_appid",
      "timeStamp": "1794967200",
      "nonceStr": "example_nonce",
      "package": "prepay_id=wx_example_prepay_id",
      "signType": "RSA",
      "paySign": "支付服务返回的签名"
    }
  }
}
```

| 字段                | 类型     | 说明                                  |
|-------------------|--------|-------------------------------------|
| `code`            | number | `0` 表示成功，其余值为错误码                    |
| `message`         | string | 成功为 `ok`，失败时返回错误说明                  |
| `data.outTradeNo` | string | 支付服务生成的支付单号，对应业务回调中的 `out_trade_no` |
| `data.tradeType`  | string | 本次下单的支付方式                           |
| `data.payParams`  | object | 对应渠道、支付方式的调起支付参数                    |

创建订单响应字段使用 `outTradeNo`、`tradeType`、`payParams`。`payParams` 内部按以下各支付方式的字段名读取。

### 调起支付参数

#### JSAPI

微信和抖音 JSAPI 返回以下字段，字段值均为字符串。

| 字段          | 说明                            |
|-------------|-------------------------------|
| `prepay_id` | 预支付交易会话标识                     |
| `appId`     | 本次支付使用的应用 AppID               |
| `timeStamp` | 秒级 Unix 时间戳                   |
| `nonceStr`  | 随机字符串                         |
| `package`   | `prepay_id=` 加预支付交易会话标识       |
| `signType`  | 微信为 `RSA`，抖音为 `DouyinPay-RSA` |
| `paySign`   | 调起支付签名                        |

微信小程序从中读取 `timeStamp`、`nonceStr`、`package`、`signType`、`paySign`，传给 `wx.requestPayment`。

支付宝 JSAPI 返回以下参数，由客户端传给 `my.tradePay`。

```json
{
  "tradeNO": "支付宝交易号"
}
```

#### NATIVE

三个渠道均通过 `code_url` 返回二维码内容，由客户端生成支付二维码。

```json
{
  "code_url": "渠道返回的二维码链接"
}
```

#### H5

三个渠道均通过 `h5_url` 返回支付链接，由浏览器打开。

```json
{
  "h5_url": "渠道返回的H5支付链接"
}
```

#### APP

微信 APP 参数如下，字段值均为字符串，客户端按平台 SDK 参数名映射。

```json
{
  "appId": "wx_example_appid",
  "partnerId": "微信支付商户号",
  "prepayId": "预支付交易会话标识",
  "packageValue": "Sign=WXPay",
  "nonceStr": "example_nonce",
  "timeStamp": "1794967200",
  "sign": "支付服务返回的签名"
}
```

抖音 APP 参数如下，字段值均为字符串。

```json
{
  "appid": "抖音应用AppID",
  "partnerid": "抖音支付商户号",
  "prepayid": "预支付交易会话标识",
  "package": "Sign=DYPay",
  "noncestr": "example_nonce",
  "timestamp": "1794967200",
  "sign": "支付服务返回的签名"
}
```

支付宝 APP 返回完整订单字符串，由客户端交给支付宝支付 SDK。

```json
{
  "order_string": "支付服务生成的完整签名订单串"
}
```

### 失败响应

```json
{
  "code": 16,
  "message": "无效的访问凭证",
  "data": {}
}
```

| HTTP 状态码 | code       | 常见含义                          |
|----------|------------|-------------------------------|
| `400`    | `3`        | 请求 JSON 或渠道下单参数错误             |
| `400`    | `9`        | 渠道配置或下单前置条件异常                 |
| `401`    | `16`       | API Key 缺失、无效或所属商户已禁用         |
| `403`    | `7`        | API Key 所属商户与请求 `account` 不一致 |
| `429`    | `8`        | 渠道请求频率受限                      |
| `500`    | `2` 或 `13` | 下单流程或服务内部异常，具体原因见 `message`   |
| `503`    | `14`       | 鉴权服务、商户配置或支付渠道暂不可用，或下单结果待核实   |
| `504`    | `4`        | 请求超时                          |

### 订单号与重复请求

`order_no` 由商户生成，`data.outTradeNo` 为支付服务返回的支付单号。同一业务订单重复调用创建订单接口可能产生多个支付单。

创建成功后保存 `data.outTradeNo`，并复用已取得的支付参数。遇到超时、网络中断或下单结果待核实时，先联系支付服务按已有支付单号核实。响应丢失时可提供 `account`、`order_no` 和请求时间协助定位。

## 支付成功业务回调

### 回调地址与请求格式

微信支付成功后，支付服务向商户提供的回调地址发送 HTTP POST。

```http
POST https://merchant.example.com/payment/notify
Content-Type: application/json
X-Pay-Timestamp: 1794967200
X-Pay-Signature: v1=<64个小写十六进制字符>
```

| 请求头               | 说明                               |
|-------------------|----------------------------------|
| `X-Pay-Timestamp` | 本次投递的秒级 Unix 时间戳，十进制字符串          |
| `X-Pay-Signature` | `v1=` 加 HMAC-SHA256 签名的小写十六进制字符串 |

### 回调参数

| 字段                 | 类型      | 说明                                                |
|--------------------|---------|---------------------------------------------------|
| `out_trade_no`     | string  | 支付服务支付单号，与创建订单响应 `data.outTradeNo` 一致，作为本次支付的幂等标识 |
| `order_no`         | string  | 业务订单号，与创建订单请求 `order_no` 一致                       |
| `merchant_account` | string  | 商户账号，与创建订单请求 `account` 一致                         |
| `trade_no`         | string  | 微信支付渠道交易号                                         |
| `channel`          | string  | 当前为 `wechat`                                      |
| `status`           | string  | 当前为 `SUCCESS`，表示支付成功                              |
| `total_amount`     | integer | 订单总金额，单位为分，例如下单 `amount = 12.50` 对应 `1250`        |
| `pay_amount`       | integer | 用户实付金额，单位为分，优惠场景下可能小于订单总金额                        |
| `pay_time`         | string  | 支付成功时间，RFC3339 格式，例如 `2026-11-18T10:00:00+08:00`  |
| `open_id`          | string  | 付款用户在对应应用下的 OpenID                                |
| `trade_type`       | string  | 微信回调中的交易类型，沿用渠道原值，H5 支付可能为 `MWEB`                 |

创建订单的 `amount` 使用元，业务回调中的 `total_amount`、`pay_amount` 使用整数分。业务关联时同时核对商户账号、业务订单号和订单总金额。

### 回调示例

```json
{
  "out_trade_no": "123456789012345678",
  "order_no": "ORDER202611180001",
  "merchant_account": "merchant_demo",
  "trade_no": "4200000000000000000000000000",
  "channel": "wechat",
  "status": "SUCCESS",
  "total_amount": 1250,
  "pay_amount": 1250,
  "pay_time": "2026-11-18T10:00:00+08:00",
  "open_id": "用户在对应AppID下的OpenID",
  "trade_type": "JSAPI"
}
```

### 签名规则

签名使用商户的 `notify_secret` 字符串 UTF-8 字节作为密钥，按以下顺序拼接待签名内容。

```text
message = "v1\n" + timestamp + "\n" + 原始请求体 + "\n"
signature = hex(HMAC-SHA256(UTF8(notify_secret), message))
X-Pay-Signature = "v1=" + signature
```

`timestamp` 使用请求头 `X-Pay-Timestamp` 的原始文本，`\n` 为单个换行字节，末尾换行参与签名。商户读取收到的原始请求体完成验签，验签通过后再解析 JSON。

### 商户验签示例

以下 Go 示例返回通过验证的原始请求体。调用时传入商户后端保存的回调密钥和 `time.Now()`。

```go
package merchant

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func VerifyNotify(req *http.Request, secret string, now time.Time) ([]byte, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	timestamp := req.Header.Get("X-Pay-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || seconds < now.Unix()-300 || seconds > now.Unix()+300 {
		return nil, errors.New("通知时间无效")
	}
	encoded, ok := strings.CutPrefix(req.Header.Get("X-Pay-Signature"), "v1=")
	if !ok {
		return nil, errors.New("签名版本无效")
	}
	actual, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("签名格式错误")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v1\n" + timestamp + "\n"))
	mac.Write(body)
	mac.Write([]byte("\n"))
	if !hmac.Equal(actual, mac.Sum(nil)) {
		return nil, errors.New("签名错误")
	}
	return body, nil
}
```

时间戳与商户当前时间相差不超过 300 秒时满足时效要求，前后相差 301 秒的通知应被拒绝。平台与商户均需维护准确时钟。

验签通过后解析返回的请求体，核对 `merchant_account`、`order_no`、`total_amount` 与业务订单。核对通过后按 `out_trade_no` 幂等处理。签名、时效或业务核对失败时返回 HTTP 非 2xx 状态码。

### 签名核对示例

以下使用虚构密钥 `notify_demo_secret_20261118` 和固定时间戳 `1794967200`。原始请求体为以下 305 字节的单行文本，报文本身末尾无换行。

```json
{"out_trade_no":"123456789012345678","order_no":"ORDER202611180001","merchant_account":"merchant_demo","trade_no":"4200000000000000000000000000","channel":"wechat","status":"SUCCESS","total_amount":1250,"pay_amount":1250,"pay_time":"2026-11-18T10:00:00+08:00","open_id":"openid_demo","trade_type":"JSAPI"}
```

期望签名头如下。

```http
X-Pay-Signature: v1=63aa64de9e6903085056f9147bc3ca589af80768e4295317eaa2b861038d193a
```

此示例用于核对签名计算，固定接收时间可使用 `time.Unix(1794967200, 0)`。实际请求使用当前投递时间。

### 接收应答与重复通知

- 业务端处理成功后返回 HTTP `200` 或 `204`，响应体可为空，支付服务根据 HTTP 状态码确认投递结果。
- 单次通知请求超时为 2 秒，`3xx` 跳转和其余非 `2xx` 状态码均按投递失败处理。
- 同一支付结果可能重复或并发通知，商户按 `out_trade_no` 幂等处理，确保同一支付单的业务效果累计为一次。已处理成功的有效重复通知返回 HTTP `200` 或 `204`。
- 每次重新投递时，支付服务重新读取当前时间并计算签名。`X-Pay-Timestamp` 为本次投递时间，`pay_time` 为原支付成功时间，业务关联使用原支付单号。同一秒内的相同报文可能得到相同签名。
- 回调接收异常或支付成功后长时间未收到回调时，请联系支付服务核实支付结果和通知情况。

成功应答示例如下。

```http
HTTP/1.1 204 No Content
```
