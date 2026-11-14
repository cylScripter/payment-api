package pay

import (
	"context"

	paymentv1 "payment-api/api/payment/v1"
)

// Payer 由每个渠道实现，单个实现提供该渠道的四种支付方式。
// 每次下单创建独立对象，保存本次请求的配置、参数及响应。
type Payer interface {
	LoadConfig(ctx context.Context) error
	BuildReq() error
	JSAPI(ctx context.Context) error
	Native(ctx context.Context) error
	H5(ctx context.Context) error
	App(ctx context.Context) error
	GetPayReq() string
	GetRawResp() string
	GetPayResp() (*paymentv1.CreateOrderResp, error)
}
