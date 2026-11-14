# 调用支付渠道下单过程

获取商户
校验商户是否存在，以及商户的支付权限。

校验请求
校验商户、订单号、金额、币种、支付方式等参数，

检查订单幂等
根据商户订单号查重，已有订单按当前状态处理，避免重复下单。

选择支付渠道
根据支付方式、商户配置、渠道状态、限额和路由规则，确定渠道及渠道商户号。

创建支付流水
生成网关支付单号和渠道请求流水号，保存本次调用记录，关联业务订单。

加载渠道配置
获取接口地址、商户号、密钥、证书、超时配置和回调地址。

转换渠道参数
将网关统一请求转换成渠道要求的字段，处理金额单位、时间格式、支付场景等差异。

发送渠道请求
调用渠道接口，记录请求耗时、通信结果及脱敏后的请求响应

返回支付参数
将渠道返回的支付参数转换成网关统一格式，返回给调用方。

## 渠道与支付方式扩展

- `PaymentHandler` 使用模板方法固定九步顺序，通过 `CreateOrder` 接口调用各步骤。
- `CreateOrderHandler` 组织公共业务，委托 `Payer` 执行配置加载、请求构建、支付调用和响应转换。
- `Payer` 显式定义 `JSAPI`、`Native`、`H5`、`App` 四种支付方式，一个渠道提供一个实现类型。
- `ChannelRegistry` 按渠道名查找工厂，为每次下单创建独立的渠道对象。

例如 `WechatPayer`、`OtherPayer` 各自实现 `Payer` 的全部方法。配置加载、参数构建和响应转换由同一个渠道对象完成，`CreateOrderHandler.ToPay` 根据支付方式调用该对象的对应方法。

各支付方法内部使用对应 SDK 的具体请求和响应类型，并将结果保存为统一响应。SDK 客户端可共享，每次下单的状态由独立对象持有。

渠道工厂签名为 `func(*pay.Order) (pay.Payer, error)`。默认注册表已注册 `wechat`、`douyin` 和 `alipay`，请求省略 `channel` 时使用微信渠道。

增加渠道时，实现 `Payer` 接口并注册一个渠道工厂。增加支付方式时，在 `Payer` 接口、参数校验和支付分发中增加对应入口。注册表构造时复制工厂映射，之后可供并发请求读取。

请求通过 `merchant_account` 加载商户，`SelectChannel` 按商户 ID 和渠道编码加载已开通的渠道配置。微信渠道从 `pay_merchant_channel.config` 加载配置，`private_key` 和 `public_key` 沿用已有工具加密存储。

微信默认使用 `channel_app_id`，为空时读取配置的 `app_id`。配置中的 `app_ids` 可按支付方式覆盖，例如 `{"JSAPI":"wx_miniprogram_appid","APP":"wx_mobile_appid"}`，APP 须使用移动应用的 AppID。H5 请求须提供 `payer_client_ip` 和 `h5_type`（`Wap`、`iOS`、`Android`）。

`WechatPayer` 已实现四种支付方式的下单、应答验签、JSAPI/APP 调起签名和统一响应。渠道下单超时或应答不可信时须使用原订单号核实结果。幂等检查命中已有订单时填充 `Order.Response`，流程直接返回该结果。

## 抖音渠道

请求设置 `channel: "douyin"`，`trade_type` 支持 `JSAPI`、`NATIVE`、`H5`、`APP`。配置来自对应商户的 `pay_merchant_channel.config`，使用普通商户接口和 RSA 签名、平台证书验签。

```json
{
  "app_id": "应用AppID",
  "app_ids": {
    "JSAPI": "网站应用AppID",
    "APP": "移动应用AppID"
  },
  "mch_id": "抖音支付商户号",
  "cert_serial_no": "商户证书序列号",
  "private_key": "helpers.EncryptPEMString生成的密文",
  "platform_cert": "-----BEGIN CERTIFICATE-----\n平台证书内容\n-----END CERTIFICATE-----",
  "notify_url": "https://example.com/payments/douyin/notify"
}
```

AppID 选择优先级为 `app_ids[trade_type]`、`channel_app_id`、`app_id`，商户号优先使用 `channel_merchant_no`。H5 需要 `payer_client_ip` 和 `h5_type`，场景类型支持 `Wap`、`iOS`、`Android`、`HarmonyOS`。

`GetPayReq` 返回完整渠道请求 JSON，`GetRawResp` 返回已获取的渠道响应体。JSAPI 返回 `signType: "DouyinPay-RSA"` 的调起参数，APP 返回对应 OpenSDK 字段和 `package: "Sign=DYPay"`，Native 和 H5 分别返回 `code_url`、`h5_url`。`notify_url` 须指向商户实际接收支付通知的接口。

接口和签名依据[抖音支付官方 Go SDK](https://github.com/douyinpay/douyinpay-go)、[JSAPI 调起支付](https://pay.douyinpay.com/wiki/639fd48f17c2f3021d237f61/64413fddc6217f024ae23ae9)和[APP 调起支付](https://pay.douyinpay.com/wiki/639fd48f17c2f3021d237f61/639fd5e470f838021f2961e5)。

## 支付宝渠道

请求设置 `channel: "alipay"`，支持 `JSAPI`、`NATIVE`、`H5`、`APP`。使用商户应用直连、公钥模式和 RSA2 签名，配置保存在对应商户的 `pay_merchant_channel.config`。

```json
{
  "app_id": "支付宝应用AppID",
  "seller_id": "收款商户PID",
  "private_key": "helpers.EncryptPEMString生成的应用私钥密文",
  "alipay_public_key": "支付宝公钥内容",
  "notify_url": "https://example.com/payments/alipay/notify",
  "return_url": "https://example.com/payment/result",
  "sandbox": false
}
```

`channel_app_id`、`channel_merchant_no` 分别优先覆盖 `app_id`、`seller_id`。`sandbox: true` 使用支付宝沙箱网关，需要配套的沙箱应用和密钥。`return_url` 是可选的 H5 同步跳转地址，`notify_url` 指向实际接收支付宝异步通知的接口。

| 支付方式 | 支付宝接口 | pay_params |
| --- | --- | --- |
| JSAPI | alipay.trade.create | `tradeNO`，交给 `my.tradePay` |
| NATIVE | alipay.trade.precreate | `code_url`，生成二维码 |
| H5 | alipay.trade.wap.pay | `h5_url`，浏览器跳转 |
| APP | alipay.trade.app.pay | `order_string`，交给支付宝客户端 SDK |

JSAPI 的 `openid` 对应当前应用下的 `buyer_open_id`。金额按元传入，最多两位小数，`attach` 经 URL 编码后作为 `passback_params` 传递，编码后最多 512 个字符。H5、APP 在本地生成签名参数，`GetRawResp` 为空，JSAPI、Native 保存完整原始响应并由 SDK 验签。

接口对应关系参考[支付宝 Easy SDK 能力列表](https://github.com/alipay/alipay-easysdk#apilist)，Go 客户端使用 [smartwalle/alipay](https://github.com/smartwalle/alipay)。
