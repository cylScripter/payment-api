package wechatpay

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"payment-api/internal/models"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewClient 使用商户渠道配置创建微信支付客户端。
func NewClient(ctx context.Context, cfg models.WeChatPayConfig) (*core.Client, error) {
	if cfg.AppID == "" || cfg.MchId == "" || cfg.CertSerialNo == "" || !strings.HasPrefix(cfg.PublicKeyId, "PUB_KEY_ID_") {
		return nil, status.Error(codes.FailedPrecondition, "微信支付配置缺少 app_id、mch_id、cert_serial_no 或有效 public_key_id")
	}
	notifyURL, err := url.Parse(cfg.NotifyUrl)
	if err != nil || notifyURL.Scheme != "https" || notifyURL.Hostname() == "" ||
		notifyURL.RawQuery != "" || notifyURL.ForceQuery || notifyURL.Fragment != "" ||
		notifyURL.User != nil || len(cfg.NotifyUrl) > 255 {
		return nil, status.Error(codes.FailedPrecondition, "微信支付 notify_url 须为不带查询参数的完整 HTTPS 地址")
	}
	privateKey, err := utils.LoadPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "加载微信支付商户私钥失败，请检查配置及解密密钥")
	}
	publicKey, err := utils.LoadPublicKey(cfg.PublicKey)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "加载微信支付公钥失败，请检查配置及解密密钥")
	}
	if privateKey.N.BitLen() != 2048 || publicKey.N.BitLen() != 2048 {
		return nil, status.Error(codes.FailedPrecondition, "微信支付密钥须为 RSA 2048 位")
	}
	client, err := core.NewClient(ctx,
		option.WithWechatPayPublicKeyAuthCipher(cfg.MchId, cfg.CertSerialNo, privateKey, cfg.PublicKeyId, publicKey),
		option.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "初始化微信支付客户端失败")
	}
	return client, nil
}
