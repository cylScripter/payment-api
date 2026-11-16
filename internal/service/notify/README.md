# 微信支付回调

HTTP 入口为 `POST /v1/wechat/pay/notify/:merchant_account`，RPC 入口为 `PaymentService.WxPayNotify`。

`WechatNotifier` 使用商户渠道中的微信支付公钥验签，使用 APIv3 密钥解密。按微信 `out_trade_no` 查找网关 `pay_no`，核对商户、应用、用户、总金额、币种和渠道交易号。

验签解密由 `internal/pkg/wechatpay.NotifyParser` 处理。使用解密后的 `WeChatPayConfig` 调用 `NewNotifyParser(cfg)` 构造，`Parse(ctx, request)` 接收包含微信请求头和原始 Body 的 `*http.Request`，返回通知信息与 `payments.Transaction`，返回后请求 Body 仍可读取。

## 数据写入

- `pay_order` 在单表事务中锁定并幂等确认支付成功，同时保存业务通知的待投递状态。
- `pay_notify_log` 独立记录每次回调和处理结果，写入失败仅记录应用错误日志。
- `pay_flow` 由资金账单流程维护。

支付订单更新成功后应答 HTTP 204。业务通知投递失败保留待投递状态，回调接收结果继续保持成功。

已有数据库需先应用 [20261116_pay_order_business_notify.sql](../../../migrations/20261116_pay_order_business_notify.sql)。新增字段的默认值为空，历史订单在收到支付成功回调后才进入业务通知流程。

## 业务通知

向 `pay_merchant.notify_url` 发送 HTTP POST，报文类型为 `application/json`。`pay_order.notify_url` 继续保存渠道回调地址。业务端以 HTTP 2xx 确认接收，单次 HTTP 请求超时为 2 秒。

```json
{
  "pay_no": "网关支付单号",
  "biz_order_no": "业务订单号",
  "merchant_account": "商户账号",
  "trade_no": "微信支付交易号",
  "channel": "wechat",
  "status": "SUCCESS",
  "total_amount": 100,
  "pay_amount": 90,
  "currency": "CNY",
  "pay_time": "2026-11-16T10:00:00+08:00"
}
```

金额均为整数分，`pay_amount` 为用户实付金额。接收方按 `pay_no` 幂等处理。服务每 5 秒扫描待投递订单，失败记录保留至后续重试。业务接收成功而投递状态保存失败时，会再次投递同一支付结果。

业务通知状态保存在支付单中，通知日志失败及进程重启均保留待投递订单。重试任务随服务启动，服务关闭时取消请求并等待任务退出。

微信字段与验签解密要求参考[支付成功回调通知](https://pay.weixin.qq.com/doc/v3/merchant/4012791861)。
