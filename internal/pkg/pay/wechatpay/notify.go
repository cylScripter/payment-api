package wechatpay

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"payment-api/internal/pkg/pay"
	"strings"

	"payment-api/internal/models"

	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	sdknotify "github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NotifyHandler 使用微信支付公钥验签、APIv3 密钥解密，可复用处理多个请求。
type NotifyHandler struct {
	handler *sdknotify.Handler
}

type NotifyResult struct {
	Notification *sdknotify.Request
	Transaction  payments.Transaction
}

func NewNotifyParser(cfg models.WeChatPayConfig) (pay.Notifier, error) {
	if len(cfg.ApiV3Key) != 32 || !strings.HasPrefix(cfg.PublicKeyId, "PUB_KEY_ID_") {
		return nil, status.Error(codes.FailedPrecondition, "微信回调配置缺少有效 APIv3 密钥或微信支付公钥 ID")
	}
	publicKey, err := utils.LoadPublicKey(cfg.PublicKey)
	if err != nil || publicKey.N.BitLen() != 2048 {
		return nil, status.Error(codes.FailedPrecondition, "加载微信支付 RSA 2048 位公钥失败")
	}
	handler, err := sdknotify.NewRSANotifyHandler(cfg.ApiV3Key,
		verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.PublicKeyId, *publicKey))
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "初始化微信回调验签解密器失败")
	}
	return &NotifyHandler{handler: handler}, nil
}

// Parse 保留请求体的原始字节供验签，并在返回后保持 Body 可读取。
func (p *NotifyHandler) Parse(ctx context.Context, request *http.Request, data interface{}) error {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}
	if request.Body == nil {
		return status.Error(codes.InvalidArgument, "微信回调请求体为空")
	}
	body, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return status.Error(codes.InvalidArgument, "读取微信回调请求体失败")
	}
	// SDK 直接解引用 resource 并调用 GCM.Open，缺少资源或 nonce 长度错误会 panic。
	var envelope sdknotify.Request
	if err := json.Unmarshal(body, &envelope); err != nil ||
		envelope.Resource == nil || len(envelope.Resource.Nonce) != 12 {
		return status.Error(codes.InvalidArgument, "微信回调报文格式错误")
	}
	result := &NotifyResult{}
	notification, err := p.handler.ParseNotifyRequest(ctx, request, &result.Transaction)
	if err != nil {
		return status.Error(codes.Unauthenticated, "微信回调验签或解密失败")
	}
	result.Notification = notification
	if _, ok := data.(*NotifyResult); ok {
		*data.(*NotifyResult) = *result
	}
	return nil
}
