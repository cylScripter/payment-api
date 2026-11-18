package wechatpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"payment-api/internal/pkg/pay"
	"strconv"
	"strings"

	"payment-api/internal/models"

	log "github.com/sirupsen/logrus"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	sdknotify "github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NotifyHandler 使用微信支付公钥验签、APIv3 密钥解密，可复用处理多个请求。
type NotifyHandler struct {
	handler  *sdknotify.Handler
	verifier *verifiers.SHA256WithRSAPubkeyVerifier
	apiV3Key string
}

var _ pay.Notifier = (*NotifyHandler)(nil)

type NotifyResult struct {
	Notification *sdknotify.Request
	Transaction  payments.Transaction
}

func NewNotifyParser(cfg models.WeChatPayConfig) (*NotifyHandler, error) {
	if len(cfg.ApiV3Key) != 32 || !strings.HasPrefix(cfg.PublicKeyId, "PUB_KEY_ID_") {
		return nil, status.Error(codes.FailedPrecondition, "微信回调配置缺少有效 APIv3 密钥或微信支付公钥 ID")
	}
	publicKey, err := utils.LoadPublicKey(cfg.PublicKey)
	if err != nil || publicKey.N.BitLen() != 2048 {
		return nil, status.Error(codes.FailedPrecondition, "加载微信支付 RSA 2048 位公钥失败")
	}
	verifier := verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.PublicKeyId, *publicKey)
	handler, err := sdknotify.NewRSANotifyHandler(cfg.ApiV3Key, verifier)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "初始化微信回调验签解密器失败")
	}
	return &NotifyHandler{handler: handler, verifier: verifier, apiV3Key: cfg.ApiV3Key}, nil
}

// Parse 保留请求体的原始字节供验签，并在返回后保持 Body 可读取。
func (p *NotifyHandler) Parse(ctx context.Context, request *http.Request, data interface{}) error {
	if _, _, err := readNotifyRequest(ctx, request); err != nil {
		return err
	}
	result := &NotifyResult{}
	notification, err := p.handler.ParseNotifyRequest(ctx, request, &result.Transaction)
	if err != nil {
		log.Errorf("[NotifyHandler]err: %v", err)
		return status.Error(codes.Unauthenticated, "微信回调验签或解密失败")
	}
	result.Notification = notification
	if _, ok := data.(*NotifyResult); ok {
		*data.(*NotifyResult) = *result
	}
	return nil
}

// ParseWithoutTimeCheck 验证原始签名并解密通知，跳过时间戳有效期检查。
func (p *NotifyHandler) ParseWithoutTimeCheck(ctx context.Context, request *http.Request, data interface{}) error {
	body, notification, err := readNotifyRequest(ctx, request)
	if err != nil {
		return err
	}
	signatureType := request.Header.Get("Wechatpay-Signature-Type")
	if signatureType != "" && signatureType != "WECHATPAY2-SHA256-RSA2048" {
		return status.Error(codes.Unauthenticated, "微信回调签名算法不支持")
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(request.Header.Get("Wechatpay-Timestamp")), 10, 64)
	nonce := strings.TrimSpace(request.Header.Get("Wechatpay-Nonce"))
	if err != nil || nonce == "" {
		return status.Error(codes.Unauthenticated, "微信回调验签失败")
	}
	message := fmt.Sprintf("%d\n%s\n%s\n", timestamp, nonce, body)
	err = p.verifier.Verify(ctx, strings.TrimSpace(request.Header.Get("Wechatpay-Serial")),
		message, strings.TrimSpace(request.Header.Get("Wechatpay-Signature")))
	if err != nil {
		return status.Error(codes.Unauthenticated, "微信回调验签失败")
	}
	resource := notification.Resource
	if resource.Algorithm != "AEAD_AES_256_GCM" {
		return status.Error(codes.Unauthenticated, "微信回调加密算法不支持")
	}
	plaintext, err := utils.DecryptAES256GCM(p.apiV3Key, resource.AssociatedData, resource.Nonce, resource.Ciphertext)
	if err != nil {
		return status.Error(codes.Unauthenticated, "微信回调解密失败")
	}
	result := &NotifyResult{Notification: &notification}
	if err := json.Unmarshal([]byte(plaintext), &result.Transaction); err != nil {
		return status.Error(codes.Unauthenticated, "微信回调解密结果格式错误")
	}
	resource.Plaintext = plaintext
	if _, ok := data.(*NotifyResult); ok {
		*data.(*NotifyResult) = *result
	}
	return nil
}

func readNotifyRequest(ctx context.Context, request *http.Request) ([]byte, sdknotify.Request, error) {
	var req sdknotify.Request
	if err := ctx.Err(); err != nil {
		return nil, req, status.FromContextError(err).Err()
	}
	if request.Body == nil {
		return nil, req, status.Error(codes.InvalidArgument, "微信回调请求体为空")
	}
	body, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return nil, req, status.Error(codes.InvalidArgument, "读取微信回调请求体失败")
	}
	// SDK 直接解引用 resource 并调用 GCM.Open，缺少资源或 nonce 长度错误会 panic。
	var envelope sdknotify.Request
	if err := json.Unmarshal(body, &envelope); err != nil ||
		envelope.Resource == nil || len(envelope.Resource.Nonce) != 12 {
		return nil, req, status.Error(codes.InvalidArgument, "微信回调报文格式错误")
	}
	return body, envelope, nil
}
