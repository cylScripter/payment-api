package controllers

import (
	"net/http"

	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/service"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type PaymentController struct {
	Controller
}

func (c PaymentController) CreateOrder(ctx *gin.Context) {
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

func (c PaymentController) WxPayNotify(ctx *gin.Context) {
	body, err := ctx.GetRawData()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"code":    "FAIL",
			"message": "读取回调请求体失败",
		})
		return
	}
	req := &paymentv1.WxPayNotifyReq{
		MerchantAccount: ctx.Param("merchant_account"),
		Body:            body,
		Headers: map[string]string{
			"Wechatpay-Serial":    ctx.GetHeader("Wechatpay-Serial"),
			"Wechatpay-Signature": ctx.GetHeader("Wechatpay-Signature"),
			"Wechatpay-Timestamp": ctx.GetHeader("Wechatpay-Timestamp"),
			"Wechatpay-Nonce":     ctx.GetHeader("Wechatpay-Nonce"),
		},
	}
	var payment service.Payment
	if _, err := payment.WxPayNotify(ctx.Request.Context(), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"code":    "FAIL",
			"message": status.Convert(err).Message(),
		})
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (c PaymentController) ReissueWxPayNotify(ctx *gin.Context) {
	data, err := ctx.GetRawData()
	if err != nil {
		c.Response(ctx, nil, status.Error(codes.InvalidArgument, "读取请求体失败"))
		return
	}
	var req paymentv1.WxPayNotifyReq
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &req); err != nil {
		c.Response(ctx, nil, status.Errorf(codes.InvalidArgument, "请求参数错误: %s", err))
		return
	}
	var payment service.Payment
	resp, err := payment.ReissueWxPayNotify(ctx.Request.Context(), &req)

	c.Response(ctx, resp, err)
}
