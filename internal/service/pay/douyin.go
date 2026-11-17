package pay

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/models"
	"payment-api/internal/pkg/helpers"

	"github.com/douyinpay/douyinpay-go/client"
	"github.com/douyinpay/douyinpay-go/services/pay/app"
	"github.com/douyinpay/douyinpay-go/services/pay/h5"
	"github.com/douyinpay/douyinpay-go/services/pay/jsapi"
	"github.com/douyinpay/douyinpay-go/services/pay/native"
	"github.com/douyinpay/douyinpay-go/tools/auth/signers"
	"github.com/douyinpay/douyinpay-go/tools/auth/verifiers"
	"github.com/douyinpay/douyinpay-go/utils"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var douyinOrderNumber = regexp.MustCompile(`^[0-9A-Za-z_*-]{6,32}$`)

// DouyinPayer 为每次下单保存独立的渠道配置、请求和响应。
type DouyinPayer struct {
	order       *Order
	config      models.DouyinPayConfig
	client      *client.Client
	signer      *signers.SHA256WithRSASigner
	appReq      app.PrepayRequest
	jsapiReq    jsapi.PrepayRequest
	nativeReq   native.PrepayRequest
	h5Req       h5.PrepayRequest
	params      proto.Message
	started     time.Time
	rawRequest  string
	rawResponse string
}

var _ Payer = (*DouyinPayer)(nil)

func NewDouyinPayer(order *Order) (Payer, error) {
	return &DouyinPayer{order: order}, nil
}

func (p *DouyinPayer) LoadConfig(ctx context.Context) error {
	channel := p.order.ChannelConfig
	cfg, err := channel.DouyinPayConfig()
	if err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if channel.ChannelAppID != "" {
		cfg.AppID = channel.ChannelAppID
	}
	if appID := cfg.AppIDs[p.order.Request.TradeType]; appID != "" {
		cfg.AppID = appID
	}
	if channel.ChannelMerchantNo != "" {
		cfg.MchID = channel.ChannelMerchantNo
	}
	if strings.TrimSpace(cfg.AppID) == "" || len(cfg.AppID) > 32 || strings.TrimSpace(cfg.MchID) == "" || len(cfg.MchID) > 32 || strings.TrimSpace(cfg.CertSerialNo) == "" {
		return status.Error(codes.FailedPrecondition, "抖音支付配置缺少有效 app_id、mch_id 或 cert_serial_no")
	}
	notifyURL, err := url.Parse(cfg.NotifyURL)
	if err != nil || notifyURL.Scheme != "https" || notifyURL.Hostname() == "" ||
		notifyURL.RawQuery != "" || notifyURL.ForceQuery || notifyURL.Fragment != "" ||
		notifyURL.User != nil || len(cfg.NotifyURL) > 256 {
		return status.Error(codes.FailedPrecondition, "抖音支付 notify_url 须为不带查询参数的完整 HTTPS 地址")
	}
	privateKey, err := utils.LoadPrivateKey(cfg.PrivateKey)
	if err != nil {
		return status.Error(codes.FailedPrecondition, "解析抖音支付商户私钥失败")
	}
	certificate, err := utils.LoadCertificate(cfg.PlatformCert)
	if err != nil {
		return status.Error(codes.FailedPrecondition, "解析抖音支付平台证书失败")
	}
	publicKey, ok := certificate.PublicKey.(*rsa.PublicKey)
	if !ok || privateKey.N.BitLen() != 2048 || publicKey.N.BitLen() != 2048 {
		return status.Error(codes.FailedPrecondition, "抖音支付商户私钥和平台证书须使用 RSA 2048 位密钥")
	}
	if now := time.Now(); now.Before(certificate.NotBefore) || now.After(certificate.NotAfter) {
		return status.Error(codes.FailedPrecondition, "抖音支付平台证书尚未生效或已过期")
	}
	signer := &signers.SHA256WithRSASigner{
		MchID:               cfg.MchID,
		CertificateSerialNo: cfg.CertSerialNo,
		PrivateKey:          privateKey,
	}
	sdkClient, err := client.NewClientWithDialSettings(ctx, &client.DialSettings{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Signer:     signer,
		Verifier: &verifiers.SHA256WithRSAVerifier{
			PublicKey:    publicKey,
			SerialNumber: utils.ConvertSerailNo(certificate.SerialNumber),
		},
	})
	if err != nil {
		return status.Error(codes.FailedPrecondition, "初始化抖音支付客户端失败")
	}
	p.config = cfg
	p.client = sdkClient
	p.signer = signer
	p.order.MerchantNo = cfg.MchID
	p.order.AppID = cfg.AppID
	p.order.NotifyURL = cfg.NotifyURL
	return nil
}

func (p *DouyinPayer) BuildReq() error {
	req := p.order.Request
	if !douyinOrderNumber.MatchString(p.order.PayNo) {
		return status.Error(codes.InvalidArgument, "抖音支付单号须为 6～32 位数字、字母或 _-* 字符")
	}
	if strings.TrimSpace(req.Description) == "" || utf8.RuneCountInString(req.Description) > 127 || utf8.RuneCountInString(req.Attach) > 1024 {
		return status.Error(codes.InvalidArgument, "description 必填且最多 127 个字符，attach 最多 1024 个字符")
	}
	if req.Amount <= 0 || math.IsNaN(float64(req.Amount)) || math.IsInf(float64(req.Amount), 0) {
		return status.Error(codes.InvalidArgument, "amount 须为大于 0 的有限金额")
	}
	amount := decimal.NewFromFloat32(req.Amount).Shift(2)
	if !amount.IsInteger() || !amount.BigInt().IsInt64() {
		return status.Error(codes.InvalidArgument, "amount 最多保留两位小数，且换算为分后须在 int64 范围内")
	}
	p.appReq = app.PrepayRequest{
		Appid:       p.config.AppID,
		Mchid:       p.config.MchID,
		Description: req.Description,
		OutTradeNo:  p.order.PayNo,
		Attach:      req.Attach,
		NotifyUrl:   p.config.NotifyURL,
		Amount: &app.Amount{
			Total:    helpers.YuanToFen(req.Amount),
			Currency: "CNY",
		},
	}
	r := p.appReq
	var request any
	switch TradeType(req.TradeType) {
	case JSAPI:
		if strings.TrimSpace(req.Openid) == "" || utf8.RuneCountInString(req.Openid) > 128 {
			return status.Error(codes.InvalidArgument, "抖音 JSAPI 支付须提供对应 appid 下的 openid")
		}
		p.jsapiReq = jsapi.PrepayRequest{
			Appid:       r.Appid,
			Mchid:       r.Mchid,
			Description: r.Description,
			OutTradeNo:  r.OutTradeNo,
			Attach:      r.Attach,
			NotifyUrl:   r.NotifyUrl,
			Amount: &jsapi.Amount{
				Total:    r.Amount.Total,
				Currency: r.Amount.Currency,
			},
			PayerInfo: &jsapi.PayerInfo{
				OpenId: req.Openid,
			},
		}
		request = p.jsapiReq
	case Native:
		p.nativeReq = native.PrepayRequest{
			Appid:       r.Appid,
			Mchid:       r.Mchid,
			Description: r.Description,
			OutTradeNo:  r.OutTradeNo,
			Attach:      r.Attach,
			NotifyUrl:   r.NotifyUrl,
			Amount: &native.Amount{
				Total:    r.Amount.Total,
				Currency: r.Amount.Currency,
			},
		}
		request = p.nativeReq
	case H5:
		if net.ParseIP(req.PayerClientIp) == nil {
			return status.Error(codes.InvalidArgument, "抖音 H5 支付须提供有效 payer_client_ip")
		}
		switch req.H5Type {
		case "Wap", "iOS", "Android", "HarmonyOS":
		default:
			return status.Error(codes.InvalidArgument, "抖音 h5_type 须为 Wap、iOS、Android 或 HarmonyOS")
		}
		p.h5Req = h5.PrepayRequest{
			Appid:       r.Appid,
			Mchid:       r.Mchid,
			Description: r.Description,
			OutTradeNo:  r.OutTradeNo,
			Attach:      r.Attach,
			NotifyUrl:   r.NotifyUrl,
			Amount: &h5.Amount{
				Total:    r.Amount.Total,
				Currency: r.Amount.Currency,
			},
			SceneInfo: &h5.SceneInfo{
				PayerClientIp: req.PayerClientIp,
				H5Info: &h5.H5Info{
					Type: req.H5Type,
				},
			},
		}
		request = p.h5Req
	case App:
		request = p.appReq
	default:
		return ErrUnsupportedPayment
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	p.rawRequest = string(data)
	return nil
}

func (p *DouyinPayer) JSAPI(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	api := jsapi.JsapiApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, p.jsapiReq)
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.PrepayId == "" {
		return status.Error(codes.Unavailable, "抖音支付应答缺少 prepay_id，请用原订单号核实结果")
	}
	return p.sign(ctx, resp.PrepayId)
}

func (p *DouyinPayer) Native(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	api := native.NativeApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, p.nativeReq)
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.CodeUrl == "" {
		return status.Error(codes.Unavailable, "抖音支付应答缺少 code_url，请用原订单号核实结果")
	}
	p.params = &paymentv1.NativePayParams{
		CodeUrl: resp.CodeUrl,
	}
	return nil
}

func (p *DouyinPayer) H5(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	api := h5.H5ApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, p.h5Req)
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.H5Url == "" {
		return status.Error(codes.Unavailable, "抖音支付应答缺少 h5_url，请用原订单号核实结果")
	}
	p.params = &paymentv1.H5PayParams{
		H5Url: resp.H5Url,
	}
	return nil
}

func (p *DouyinPayer) App(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	api := app.AppApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, p.appReq)
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.PrepayId == "" {
		return status.Error(codes.Unavailable, "抖音支付应答缺少 prepay_id，请用原订单号核实结果")
	}
	return p.sign(ctx, resp.PrepayId)
}

func (p *DouyinPayer) sign(ctx context.Context, prepayID string) error {
	nonce, err := utils.GenerateNonce()
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	packageValue := "prepay_id=" + prepayID
	if TradeType(p.order.Request.TradeType) == App {
		packageValue = prepayID
	}
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n", p.config.AppID, timestamp, nonce, packageValue)
	signature, err := p.signer.Sign(ctx, message)
	if err != nil {
		return err
	}
	if TradeType(p.order.Request.TradeType) == App {
		p.params = &paymentv1.DouyinAppPayParams{
			AppId:     p.config.AppID,
			PartnerId: p.config.MchID,
			PrepayId:  prepayID,
			Package:   "Sign=DYPay",
			NonceStr:  nonce,
			TimeStamp: timestamp,
			Sign:      signature.Signature,
		}
		return nil
	}
	p.params = &paymentv1.JSAPIPayParams{
		AppId:     p.config.AppID,
		PrepayId:  prepayID,
		TimeStamp: timestamp,
		NonceStr:  nonce,
		Package:   packageValue,
		SignType:  "DouyinPay-RSA",
		PaySign:   signature.Signature,
	}
	return nil
}

func (p *DouyinPayer) finish(ctx context.Context, result *client.APIResult, err error) error {
	p.rawResponse = ""
	requestID, httpStatus := "", 0
	if result != nil && result.Response != nil {
		requestID = result.Response.Header.Get("Request-Id")
		httpStatus = result.Response.StatusCode
		if result.Response.Body != nil {
			defer result.Response.Body.Close()
			body, readErr := io.ReadAll(result.Response.Body)
			p.rawResponse = string(body)
			if err == nil && readErr != nil {
				err = readErr
			}
		}
	}
	apiCode := ""
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		apiCode = apiErr.Code
	}
	log.WithContext(ctx).WithFields(log.Fields{
		"out_trade_no": p.order.PayNo,
		"trade_type":   p.order.Request.TradeType,
		"elapsed":      time.Since(p.started),
		"request_id":   requestID,
		"http_status":  httpStatus,
		"code":         apiCode,
		"success":      err == nil,
		"error_type":   fmt.Sprintf("%T", err),
	}).Info("douyinpay order")
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return status.FromContextError(ctx.Err()).Err()
	}
	if apiErr != nil {
		code := codes.Unavailable
		switch apiErr.StatusCode {
		case 400:
			code = codes.InvalidArgument
		case 401, 403:
			code = codes.FailedPrecondition
		case 429:
			code = codes.ResourceExhausted
		}
		return status.Errorf(code, "抖音支付下单返回错误码 %s", apiErr.Code)
	}
	return status.Error(codes.Unavailable, "未获得可信抖音下单结果，请通过原商户订单号核实，重试时保持原单号和全部参数")
}

func (p *DouyinPayer) GetPayReq() string {
	return p.rawRequest
}

func (p *DouyinPayer) GetRawResp() string {
	return p.rawResponse
}

func (p *DouyinPayer) GetPayResp() (*paymentv1.CreateOrderResp, error) {
	if p.params == nil {
		return nil, status.Error(codes.FailedPrecondition, "尚未取得抖音支付参数")
	}
	data, err := protojson.Marshal(p.params)
	if err != nil {
		return nil, err
	}
	params := &structpb.Struct{}
	if err := protojson.Unmarshal(data, params); err != nil {
		return nil, err
	}
	return &paymentv1.CreateOrderResp{
		OutTradeNo: p.order.PayNo,
		TradeType:  p.order.Request.TradeType,
		PayParams:  params,
	}, nil
}
