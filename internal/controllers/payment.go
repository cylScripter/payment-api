package controllers

import (
	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/service"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func (c Controller) CreateOrder(ctx *gin.Context) {
	data, err := ctx.GetRawData()
	if err != nil {
		c.Response(ctx, nil, status.Error(codes.InvalidArgument, "读取请求体失败"))
		return
	}
	var req paymentv1.CreateOrderReq
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &req); err != nil {
		c.Response(ctx, nil, status.Errorf(codes.InvalidArgument, "请求参数错误: %s", err))
		return
	}
	var payment service.Payment
	resp, err := payment.CreateOrder(ctx.Request.Context(), &req)
	c.Response(ctx, resp, err)
}
