package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	authv1 "payment-api/api/auth/v1"
	"payment-api/internal/models"
	"payment-api/internal/pkg/wechat"
	"payment-api/internal/repo"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Auth struct {
	authv1.UnimplementedAuthServiceServer
}

func (c *Auth) GenerateAPIKey(ctx context.Context, req *authv1.GenerateAPIKeyReq) (*authv1.GenerateAPIKeyResp, error) {
	if strings.TrimSpace(req.MerchantAccount) == "" || utf8.RuneCountInString(req.MerchantAccount) > 128 {
		return nil, status.Error(codes.InvalidArgument, "merchant_account 必填且不能超过 128 个字符")
	}
	var random [16]byte
	_, _ = rand.Read(random[:])
	key := "sk-" + hex.EncodeToString(random[:])
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	query := repo.NewReqCreateAPIKey(
		repo.WithReqCreateAPIKeyAccount(req.MerchantAccount),
		repo.WithReqCreateAPIKeyHash(keyHash),
	)
	merchant, err := repo.NewMySQLAPIKey(models.PayDb()).Create(ctx, query)
	var data []byte
	if err == nil {
		data, err = json.Marshal(merchant)
	}

	redisClient := models.RedisClient()
	if err == nil {
		err = redisClient.Set(models.APIKeyCachePrefix+keyHash, data, models.APIKeyCacheTTL)
	}
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "密钥签发失败")
	}
	return &authv1.GenerateAPIKeyResp{
		ApiKey:          key,
		MerchantAccount: merchant.Account,
	}, nil
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
