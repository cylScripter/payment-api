# 验证指南 — 商户支付回调签名

**计划** [plan.md](plan.md) | **协议** [contracts/business-notify.md](contracts/business-notify.md)

以下步骤用于构建和联调。已执行的本地验证及待完成的外部商户联调见末尾验收记录。

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

## 6. 本次实现验收记录

**日期** 2026-11-18

### 实现与执行环境

已实现商户密文字段、主库条件初始化、商户密钥服务、本地分配命令、实际通知签名和商户验签文档。

数据库验证使用本次新建的 MySQL 8.4 临时容器，绑定本机回环地址，数据为虚构测试商户及订单。HTTP 接收端运行于本机，使用商户文档的 Go 验签函数和测试用业务计数器。验证结束后临时容器及仓库内临时 Go 验证目录已清理。

### 执行结果

| 检查 | 实际结果 |
| --- | --- |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过 |
| `go test ./internal/pkg/helpers` | 通过 |
| 初始回归检查 | 实现前的发送函数缺少签名，本地接收端返回 401，符合预期失败 |
| 实际代码隔离验证 | 临时 Go overlay 暴露实际发送及 `NotifyBusiness` 调用入口，使用 `go run -race -overlay <临时映射> <临时验证包>` 运行，通过且竞态检测无报告 |
| 新旧数据库结构 | 原有建表结构应用新增迁移通过，全新 `init.sql` 建表通过，两者字段类型、非空约束及空字符串默认值一致 |
| 密钥分配 | 两个商户的密钥不同，密文可解密，重复调用返回同一密钥 |
| 并发初始化 | 同一新商户 8 个并发 service 调用返回同一密钥；真实 CLI 的 2 个并发进程返回同一保存值 |
| 命令输出与失败 | CLI 正确输出商户账号和密钥 JSON，重复执行稳定，缺账号和未知商户以非 0 退出 |
| JSON 与日志 | 商户 JSON 排除密钥字段，鉴权缓存沿用同一商户序列化方式；本次密钥分配日志中完整明文与密文检出数为 0 |
| 真实发送报文 | 捕获实际发送函数的 HTTP 请求，body 与一次 JSON 序列化结果字节一致，签名与接收端计算一致 |
| 密钥不可用 | 空密钥、损坏密文和可解密的空明文均返回投递错误，接收端收到的未签名通知数量为 0 |
| 固定向量 | 305 字节固定报文的签名核对通过，直接编译文档中的 Go 验签函数也验证通过 |
| 本地接收协议 | 错密钥、报文空白/金额修改、时间戳篡改、缺失签名、错误十六进制和不支持版本均拒绝 |
| 本地时间边界 | 使用文档验签函数和固定接收时钟，前后各 300 秒通过，301 秒拒绝 |
| 本地业务核对 | 有效签名下商户账号、订单号或总金额错误均被测试接收端拒绝 |
| 本地重复通知 | 实际发送函数串行发送 10 次、并发发送 10 次，测试接收端按支付单号处理的业务效果累计为一次 |
| 实际通知状态 | 真实 `NotifyBusiness` 在成功应答后保存通知 `SUCCESS`，503、超过 2 秒超时和缺密钥均保留通知 `PENDING`，支付状态保持 `SUCCESS` |
| 重新投递 | 本地接收端拒绝模拟过期通知后，真实发送路径读取本次时间重新签名，原支付单号及业务报文保持一致，接收端验证通过 |
| 独立代码审查 | 未发现实质缺陷，固定向量独立复算一致 |
| GitNexus `detect_changes(scope: all)` | 已执行，风险为 high，影响集中在共享商户模型、商户配置读取与通知链路；新增未跟踪文件由源码审阅和构建覆盖 |
| `git diff --check` | 通过 |

### 规格核对

| 规格范围 | 当前证据 |
| --- | --- |
| FR-1～FR-5、FR-11 | 数据库、实际 CLI、实际发送函数及原始请求体核对通过 |
| FR-6～FR-8 | 文档验签函数及本地参考接收端验证通过，外部商户接收端待联调 |
| FR-9 | 商户文档和本地参考接收端幂等行为已核对，外部商户真实业务效果待核实 |
| FR-10 | 实际发送路径每次重新计算当前时间和签名已验证，外部商户重新投递链路待联调 |
| FR-12 | 商户文档已包含密钥、签名规则、可编译验签示例、固定向量和幂等/应答约定 |
| SC-1、SC-2 | 本地商户密钥、加密保存、隔离性和固定向量通过 |
| SC-3～SC-7 | 本地发送方、文档验签函数及参考接收端场景通过，商户实际接收端业务结果待核实 |
| SC-8 | 商户文档和示例已验证，外部商户联调待完成 |

### 待完成的外部联调

任务 T010、T012、T013、T015、T016 保留未完成标记，其本地验证部分如上表所列。完成这些任务需要商户测试回调地址、可核对的测试订单、业务处理次数查询方式，以及对应环境的支付测试链路。实际密钥通过本地配置和受控交付流程传递。

取得环境后，按第 3～5 节对外部商户接收端执行验收，核对真实支付回调、真实幂等业务效果及重新投递结果，并更新对应任务状态。

发布依次执行字段迁移、目标商户密钥分配与交付、商户接收端验签准备、支付服务签名代码部署。未配置可用回调密钥的商户会产生投递失败，应在部署前完成密钥准备。
