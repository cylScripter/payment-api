package service

import (
	"context"
	"errors"
	"strings"

	authv1 "payment-api/api/auth/v1"
	"payment-api/internal/pkg/wechat"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Auth struct {
	authv1.UnimplementedAuthServiceServer
}

func (c *Auth) WechatLogin(ctx context.Context, req *authv1.WechatLoginRequest) (*authv1.WechatLoginResponse, error) {
	if strings.TrimSpace(req.Code) == "" {
		return nil, status.Error(codes.InvalidArgument, "code 必填，请传入小程序 wx.login 返回的 code")
	}
	openid, err := wechat.ExchangeCode(ctx, req.Code)
	if err == nil {
		return &authv1.WechatLoginResponse{Openid: openid}, nil
	}
	if errors.Is(err, wechat.ErrNotConfigured) {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if apiErr, ok := errors.AsType[*wechat.APIError](err); ok {
		code := codes.Unavailable
		if apiErr.Code == 40029 || apiErr.Code == 40163 {
			code = codes.Unauthenticated
		}
		return nil, status.Error(code, apiErr.Error())
	}
	return nil, status.Error(codes.Unavailable, err.Error())
}
