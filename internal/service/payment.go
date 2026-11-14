package service

import (
	"context"
	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/service/pay"
	"regexp"
)

var merchantOrderNumber = regexp.MustCompile(`^[0-9A-Za-z_|*-]{6,32}$`)

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
