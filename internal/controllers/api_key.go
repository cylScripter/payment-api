package controllers

import (
	authv1 "payment-api/api/auth/v1"
	"payment-api/internal/service"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func (c Controller) GenerateAPIKey(ctx *gin.Context) {
	data, err := ctx.GetRawData()
	if err != nil {
		c.Response(ctx, nil, status.Error(codes.InvalidArgument, "读取请求体失败"))
		return
	}
	var req authv1.GenerateAPIKeyReq
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &req); err != nil {
		c.Response(ctx, nil, status.Error(codes.InvalidArgument, "请求参数错误"))
		return
	}
	var auth service.Auth
	resp, err := auth.GenerateAPIKey(ctx.Request.Context(), &req)
	c.Response(ctx, resp, err)
}
