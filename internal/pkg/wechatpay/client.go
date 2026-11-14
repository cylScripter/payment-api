package wechatpay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"payment-api/internal/models"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
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

func CreateJSAPIOrder(ctx context.Context, req jsapi.PrepayRequest, cfg models.WeChatPayConfig) (*jsapi.PrepayWithRequestPaymentResponse, error) {
	sdkClient, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	req.Appid = core.String(cfg.AppID)
	req.Mchid = core.String(cfg.MchId)
	req.NotifyUrl = core.String(cfg.NotifyUrl)
	api := jsapi.JsapiApiService{Client: sdkClient}
	resp, result, err := api.Prepay(ctx, req)
	requestID := ""
	statusCode := 0
	if result != nil && result.Response != nil {
		defer result.Response.Body.Close()
		requestID = result.Response.Header.Get("Request-ID")
		statusCode = result.Response.StatusCode
	}
	code := ""
	message := ""
	var detail map[string]any
	if apiErr, ok := errors.AsType[*core.APIError](err); ok {
		code = apiErr.Code
		message = apiErr.Message
		detail, _ = apiErr.Detail.(map[string]any)
	}
	if err == nil && (resp.PrepayId == nil || *resp.PrepayId == "") {
		err = errors.New("微信支付应答缺少 prepay_id")
	}
	slog.InfoContext(ctx, "wechatpay JSAPI order",
		"out_trade_no", *req.OutTradeNo,
		"request_id", requestID,
		"http_status", statusCode,
		"code", code,
		"message", message,
		"field", detail["field"],
		"issue", detail["issue"],
		"location", detail["location"],
		"success", err == nil,
		"error_type", fmt.Sprintf("%T", err),
	)
	if err != nil {
		return nil, err
	}
	nonce, err := utils.GenerateNonce()
	if err != nil {
		return nil, fmt.Errorf("生成支付随机字符串失败: %w", err)
	}
	payment := &jsapi.PrepayWithRequestPaymentResponse{
		PrepayId:  resp.PrepayId,
		Appid:     req.Appid,
		TimeStamp: core.String(strconv.FormatInt(time.Now().Unix(), 10)),
		NonceStr:  core.String(nonce),
		Package:   core.String("prepay_id=" + *resp.PrepayId),
		SignType:  core.String("RSA"),
	}
	messageToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n", *payment.Appid, *payment.TimeStamp, *payment.NonceStr, *payment.Package)
	signature, err := sdkClient.Sign(ctx, messageToSign)
	if err != nil {
		return nil, fmt.Errorf("生成调起支付签名失败: %w", err)
	}
	payment.PaySign = core.String(signature.Signature)
	return payment, nil
}
