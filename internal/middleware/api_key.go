package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"payment-api/internal/models"
	merchantcache "payment-api/internal/service/merchant"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func Auth(response func(*gin.Context, proto.Message, error), checkMerchant bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		values := ctx.Request.Header.Values("Authorization")
		key, err := BearerToken(values)
		if err != nil {
			ctx.Abort()
			response(ctx, nil, err)
			return
		}
		merchant, err := Authenticate(ctx.Request.Context(), key)
		if err != nil {
			ctx.Abort()
			response(ctx, nil, err)
			return
		}
		if checkMerchant {
			body, err := ctx.GetRawData()
			if err != nil {
				ctx.Abort()
				response(ctx, nil, status.Error(codes.InvalidArgument, "读取请求体失败"))
				return
			}
			_ = ctx.Request.Body.Close()
			var params map[string]json.RawMessage
			err = json.Unmarshal(body, &params)
			value := params["account"]
			if len(value) == 0 {
				value = params["merchantAccount"]
			}
			var account string
			if err == nil {
				err = json.Unmarshal(value, &account)
			}
			if err != nil || account == "" || params["account"] != nil && params["merchantAccount"] != nil {
				ctx.Abort()
				response(ctx, nil, status.Error(codes.InvalidArgument, "请求参数错误"))
				return
			}
			if account != merchant.Account {
				ctx.Abort()
				response(ctx, nil, status.Error(codes.PermissionDenied, "无权访问该商户"))
				return
			}
			if merchantcache.GetMerchant(account).ID != merchant.ID {
				ctx.Abort()
				response(ctx, nil, status.Error(codes.Unavailable, "商户配置正在更新，请稍后重试"))
				return
			}
			ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		ctx.Next()
	}
}

func BearerToken(values []string) (string, error) {
	if len(values) != 1 {
		return "", status.Error(codes.Unauthenticated, "无效的访问凭证")
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.Contains(parts[1], ",") {
		return "", status.Error(codes.Unauthenticated, "无效的访问凭证")
	}
	return parts[1], nil
}

func Authenticate(ctx context.Context, key string) (models.PayMerchant, error) {
	var merchant models.PayMerchant
	if err := ctx.Err(); err != nil {
		return merchant, status.FromContextError(err).Err()
	}

	hash := sha256.Sum256([]byte(key))
	client := models.RedisClient()
	data, err := redis.Bytes(client.DoContext(ctx, "GET", models.APIKeyCachePrefix+hex.EncodeToString(hash[:])))
	if errors.Is(err, redis.ErrNil) {
		if _, readyErr := redis.String(client.DoContext(ctx, "GET", models.APIKeyCachePrefix+"ready")); readyErr != nil {
			err = errors.New("鉴权缓存未就绪")
		}
	}
	if err == nil {
		err = json.Unmarshal(data, &merchant)
	}
	if ctx.Err() != nil {
		return merchant, status.FromContextError(ctx.Err()).Err()
	}
	if errors.Is(err, redis.ErrNil) {
		return merchant, status.Error(codes.Unauthenticated, "无效的访问凭证")
	}
	if err != nil {
		return merchant, status.Error(codes.Unavailable, "鉴权服务暂不可用")
	}
	if merchant.Status != 1 {
		return merchant, status.Error(codes.Unauthenticated, "无效的访问凭证")
	}
	return merchant, nil
}
