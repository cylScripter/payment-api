package service

import (
	"context"
	"time"

	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/service/notify"
	"payment-api/internal/service/pay"

	"github.com/cloudwego/kitex/tool/internal_pkg/log"
)

type Payment struct {
	paymentv1.UnimplementedPaymentServiceServer
}

func (c *Payment) CreateOrder(ctx context.Context, req *paymentv1.CreateOrderReq) (*paymentv1.CreateOrderResp, error) {
	handler := pay.CreateOrderHandler{}
	handler.Build(
		pay.WithRequest(req),
		pay.WithChannel(req.Channel),
	)
	return pay.NewPaymentHandler(&handler).CreateOrder(ctx)
}

func (c *Payment) WxPayNotify(ctx context.Context, req *paymentv1.WxPayNotifyReq) (*paymentv1.WxPayNotifyResp, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	log.Infof("[WxPayNotify] 收到微信支付回调请求: %+v", req)
	notifier := notify.NewWechatNotifier(req)
	err := notify.Handle(ctx, notifier)
	if err != nil {
		log.Infof("[WxPayNotify] 处理微信支付回调请求失败: %+v", err)
		return &paymentv1.WxPayNotifyResp{}, nil
	}
	return &paymentv1.WxPayNotifyResp{}, nil
}
