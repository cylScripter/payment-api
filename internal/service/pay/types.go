package pay

import (
	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/models"
)

type TradeType string

const (
	JSAPI  TradeType = "JSAPI"
	Native TradeType = "NATIVE"
	H5     TradeType = "H5"
	App    TradeType = "APP"
)

// Order 由单次下单流程和渠道策略共享，保存路由结果及生成的支付单号。
type Order struct {
	Request       *paymentv1.CreateOrderReq
	PayNo         string
	RequestNo     string
	Channel       string
	MerchantNo    string
	AppID         string
	NotifyURL     string
	ChannelConfig models.PayMerchantChannel
	Response      *paymentv1.CreateOrderResp // 已有订单的统一响应，由幂等检查填充。
}
