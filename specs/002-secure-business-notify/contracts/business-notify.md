# 商户回调与密钥分配契约

**规格** [spec.md](../spec.md) | **模型** [data-model.md](../data-model.md)

## 本地密钥分配

本地运维入口规划为以下命令，在项目根目录执行，使用已有配置和数据库连接。

```bash
CONFIG_PATH=/path/to/config.yaml go run ./cmd/notify-secret -account merchant_demo
```

`account` 为已存在的商户账号。首次执行初始化密钥，重复执行返回该商户已保存的密钥。命令通过受控终端向授权操作者输出以下 JSON，用于现有商户交付流程。

```json
{
  "merchant_account": "merchant_demo",
  "notify_secret": "仅在密钥交付时返回的商户独立密钥"
}
```

成功退出码为 0。商户查询、配置、加解密或持久化失败时退出码非 0，标准错误输出可排查原因。交付结果与实际保存的密文对应，错误输出避开完整明文及密文。命令遵循主库条件初始化与读回约定。

## 支付成功回调

支付服务向商户提供的回调地址发送 HTTP POST。

```http
Content-Type: application/json
X-Pay-Timestamp: 1794967200
X-Pay-Signature: v1=<64个小写十六进制字符>
```

| 字段 | 含义 |
| --- | --- |
| `out_trade_no` | 支付单号，对应下单响应 `data.outTradeNo` |
| `order_no` | 商户业务订单号 |
| `merchant_account` | 商户账号 |
| `trade_no` | 渠道交易号 |
| `channel` | 当前通知渠道为 `wechat` |
| `status` | 当前为 `SUCCESS` |
| `total_amount` | 订单总金额，整数分 |
| `pay_amount` | 实付金额，整数分 |
| `pay_time` | 支付成功时间，RFC3339 字符串 |
| `open_id` | 付款人标识 |
| `trade_type` | 渠道通知的支付类型 |

### 签名规则

```text
message = "v1\n" + timestamp + "\n" + raw_body + "\n"
signature = hex(HMAC-SHA256(UTF8(notify_secret), message))
```

`timestamp` 为请求头中的十进制秒级 Unix 时间文本。`raw_body` 为实际发送的原始字节。`\n` 为单个换行字节，末尾换行参与签名。发送方对 `json.Marshal(payload)` 的结果签名并发送同一份字节，接收方读取原始请求体后验签。

### 固定签名向量

以下数据均为虚构测试值。

```text
notify_secret = notify_demo_secret_20261118
timestamp = 1794967200
```

原始报文为以下单行 305 字节文本，报文本身末尾无换行。

```json
{"out_trade_no":"123456789012345678","order_no":"ORDER202611180001","merchant_account":"merchant_demo","trade_no":"4200000000000000000000000000","channel":"wechat","status":"SUCCESS","total_amount":1250,"pay_amount":1250,"pay_time":"2026-11-18T10:00:00+08:00","open_id":"openid_demo","trade_type":"JSAPI"}
```

期望签名头如下。

```http
X-Pay-Signature: v1=63aa64de9e6903085056f9147bc3ca589af80768e4295317eaa2b861038d193a
```

### 商户验签与应答

1. 读取原始报文及两个签名请求头，识别 `v1` 版本和十六进制签名。
2. 校验通知时间与当前时间的绝对差值，差值小于或等于 300 秒时满足时效要求。
3. 使用自己的回调密钥重新计算 HMAC，将请求签名解码为字节后通过 `hmac.Equal` 比较。
4. 校验通过后解析业务报文，核对商户账号、业务订单号和订单总金额。金额按整数分核对。
5. 按 `out_trade_no` 幂等处理业务，处理成功及已处理成功的有效重复通知返回 HTTP 200 或 204。

验证或业务处理失败时返回 HTTP 非 2xx 状态，支付服务按已有投递失败流程处理。验证窗口内的重复通知通过商户业务幂等处理约束。

### 重新投递

每次实际发送重新取得当前时间并计算签名。同一支付结果使用原支付单号和业务信息，`pay_time` 表示原支付成功时间。相同秒内相同报文可能产生相同签名。固定签名向量的时间仅用于计算一致性核对，实时联调用当前时间生成签名。
