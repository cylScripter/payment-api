# 验证指南 — 商户支付回调签名

**计划** [plan.md](plan.md) | **协议** [contracts/business-notify.md](contracts/business-notify.md)

以下步骤供功能实现后的构建和联调使用。规划阶段已核对固定签名向量，命令分配密钥及实际回调链路在实现阶段验收。

## 环境准备

- Go 1.26.0、Python 3、OpenSSL 和 curl。
- 测试数据库已应用计划中的商户字段迁移，配置包含可用的数据库连接及 `encrypt_key`。
- 已存在测试商户及可用支付渠道，取得该商户下单 API Key。
- 商户测试接收端已经实现 [回调契约](contracts/business-notify.md) 中的验签、订单核对和业务幂等，并提供可查询的测试订单业务处理次数。

所有项目命令在项目根目录执行。

## 1. 构建与密钥初始化

```bash
go build ./...
go vet ./...
go test ./internal/pkg/helpers
CONFIG_PATH=/path/to/test-config.yaml go run ./cmd/notify-secret -account merchant_demo
```

预期构建、静态检查及现有加解密测试通过。密钥命令返回所属商户账号和 `notify_secret`，持久化字段为可解密的密文。将命令返回的明文通过现有接入流程交付商户测试后端。

再次执行同一商户的命令，预期取得相同密钥。对新测试商户并发执行两次，预期两次成功结果与主库最终保存值相同。商户不存在或配置不可用时，命令退出码非 0。

## 2. 固定签名向量核对

使用协议中的虚构密钥和固定时间，验证以下完整报文的签名。临时目录保存虚构测试报文。

```bash
export NOTIFY_CHECK_DIR=$(mktemp -d /private/tmp/payment-notify-check.XXXXXX)
printf '%s' '{"out_trade_no":"123456789012345678","order_no":"ORDER202611180001","merchant_account":"merchant_demo","trade_no":"4200000000000000000000000000","channel":"wechat","status":"SUCCESS","total_amount":1250,"pay_amount":1250,"pay_time":"2026-11-18T10:00:00+08:00","open_id":"openid_demo","trade_type":"JSAPI"}' > "$NOTIFY_CHECK_DIR/body.json"
python3 - <<'PY'
import hashlib
import hmac
import os
from pathlib import Path

body = (Path(os.environ['NOTIFY_CHECK_DIR']) / 'body.json').read_bytes()
assert len(body) == 305
message = b'v1\n1794967200\n' + body + b'\n'
signature = hmac.new(b'notify_demo_secret_20261118', message, hashlib.sha256).hexdigest()
assert signature == '63aa64de9e6903085056f9147bc3ca589af80768e4295317eaa2b861038d193a'
print('固定签名向量通过')
PY
```

固定时间用于计算一致性检查。实时接收端的 5 分钟时间校验使用当前时间。

## 3. 商户接收协议联调

为商户测试接收端单独准备 `merchant_demo` 和 `ORDER202611180001`，订单总金额为 1250 分，演示回调密钥配置为 `notify_demo_secret_20261118`。此步骤使用上一步的虚构数据，通知地址填写测试接收端地址。

```bash
export MERCHANT_NOTIFY_URL='http://127.0.0.1:18080/payment/notify'
notify_check_timestamp=$(date +%s)
notify_check_signature=$(python3 - "$notify_check_timestamp" "$NOTIFY_CHECK_DIR/body.json" <<'PY'
import hashlib
import hmac
import sys
from pathlib import Path

message = b'v1\n' + sys.argv[1].encode('ascii') + b'\n' + Path(sys.argv[2]).read_bytes() + b'\n'
print(hmac.new(b'notify_demo_secret_20261118', message, hashlib.sha256).hexdigest())
PY
)
curl --silent --show-error --output "$NOTIFY_CHECK_DIR/response.txt" --write-out '%{http_code}\n' \
  --request POST "$MERCHANT_NOTIFY_URL" \
  --header 'Content-Type: application/json' \
  --header "X-Pay-Timestamp: $notify_check_timestamp" \
  --header "X-Pay-Signature: v1=$notify_check_signature" \
  --data-binary "@$NOTIFY_CHECK_DIR/body.json"
```

预期商户返回 200 或 204，对应业务效果为一次。演示密钥与第一步实际初始化的商户密钥分别用于本节协议联调和下一节实际发送链路。

在上述签名仍处于有效时间内时重复发送 10 次，并发发送 10 次，预期该支付单业务效果累计为一次，各有效重复通知返回成功。可对最后一条 curl 命令使用 `xargs -P 10` 并发执行，并核对接收端业务结果。

## 4. 实际发送链路

将真实测试商户的密钥配置到其测试接收端，将业务回调地址配置为该接收端地址。使用测试配置启动支付服务。

```bash
CONFIG_PATH=/path/to/test-config.yaml go run ./cmd/server
```

在测试环境按照 [商户对接文档](../../docs/payment-integration.md) 调用 `POST /merchant-api/v1/create/order`，完成一笔授权测试支付，核对以下结果。

1. 捕获商户接收端收到的两个签名头及原始 body，按实际商户密钥计算签名，结果与请求头一致。
2. 回调 `out_trade_no` 等于下单响应 `data.outTradeNo`，`order_no` 与商户业务订单一致，`total_amount` 等于下单元金额换算后的整数分。
3. 商户完成验签和业务处理后返回 200 或 204，支付服务保存通知成功状态。
4. 在测试接收端模拟超时或返回非 2xx 应答，预期支付服务保留待通知状态，已确认支付结果继续有效。
5. 用同一有效渠道通知再次触发现有回调处理入口时，核对每次实际发送均按本次时间重新计算签名。若支付单已完成业务通知，可选用通知尚待成功的测试支付单验收重新投递。

## 5. 失败与边界场景

| 场景 | 操作 | 预期结果 |
| --- | --- | --- |
| 错误密钥 | 对同一报文使用另一商户密钥生成签名 | 验签失败，业务处理次数为 0 |
| 报文篡改 | 保持原签名，修改金额或增加报文空白后发送 | 验签失败，业务处理次数为 0 |
| 错误凭证 | 移除签名头、修改版本或传入错误十六进制值 | 接收端拒绝处理 |
| 过期或未来通知 | 分别用当前时间前后各 301 秒重新签名并发送 | 时效校验失败 |
| 精确时间边界 | 在接收端固定时钟，分别使用前后 300 秒和 301 秒的时间戳 | 300 秒通过时效检查，301 秒拒绝 |
| 业务不匹配 | 用有效签名发送与预置订单账号、订单号或总金额不符的报文 | 业务核对失败 |
| 密钥缺失 | 对空密钥测试商户触发待发通知 | 投递失败，接收端收到的未签名请求数为 0 |
| 密文无法解密 | 使用受控测试数据制造不可用密文后触发通知 | 投递失败，错误原因可排查 |
| 原请求过期后重投 | 重新触发待通知支付结果的发送路径 | 读取本次发送时间并重新计算签名，接收端可验证 |

边界时效验收采用固定接收时间，避免命令执行与网络耗时改变恰好 300 秒的预期结果。验收完成后核对 [spec.md](spec.md) 的 SC-1～SC-8，并记录实际功能结果及待验证项。
