package pay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/models"
	"payment-api/internal/pkg/wechatpay"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/app"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/h5"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var wechatOrderNumber = regexp.MustCompile(`^[0-9A-Za-z_|*-]{6,32}$`)

// WechatPayer 保存一次下单的配置、公共请求字段和调起支付参数。
type WechatPayer struct {
	order       *Order
	config      models.WeChatPayConfig
	client      *core.Client
	request     native.PrepayRequest
	params      proto.Message
	started     time.Time
	rawRequest  string
	rawResponse string
}

var _ Payer = (*WechatPayer)(nil)

func NewWechatPayer(order *Order) (Payer, error) {
	return &WechatPayer{order: order}, nil
}

func (p *WechatPayer) GetPayReq() string {
	return p.rawRequest
}

// GetRawResp 返回渠道原始响应体，包含成功或失败应答。
func (p *WechatPayer) GetRawResp() string {
	return p.rawResponse
}

func (p *WechatPayer) LoadConfig(ctx context.Context) error {
	channel := p.order.ChannelConfig
	cfg := channel.WeChatPayConfig()
	if channel.ChannelAppID != "" {
		cfg.AppID = channel.ChannelAppID
	}
	if appID := cfg.AppIDs[p.order.Request.TradeType]; appID != "" {
		cfg.AppID = appID
	}
	if channel.ChannelMerchantNo != "" {
		cfg.MchId = channel.ChannelMerchantNo
	}
	client, err := wechatpay.NewClient(ctx, cfg)
	if err != nil {
		return err
	}
	p.config, p.client = cfg, client
	p.order.MerchantNo = cfg.MchId
	p.order.AppID = cfg.AppID
	p.order.NotifyURL = cfg.NotifyUrl
	return nil
}

func (p *WechatPayer) BuildReq() error {
	req := p.order.Request
	if !wechatOrderNumber.MatchString(req.OrderNo) {
		return status.Error(codes.InvalidArgument, "order_no 须为 6～32 位数字、字母或 _-|* 字符")
	}
	if strings.TrimSpace(req.Description) == "" || utf8.RuneCountInString(req.Description) > 127 || utf8.RuneCountInString(req.Attach) > 128 {
		return status.Error(codes.InvalidArgument, "description 必填且最多 127 个字符，attach 最多 128 个字符")
	}
	for _, value := range []string{req.Description, req.Attach, req.Openid} {
		for _, char := range value {
			if char > 0xFFFF {
				return status.Error(codes.InvalidArgument, "微信支付字段须使用 UTF-8 一至三个字节编码的字符")
			}
		}
	}
	// 先恢复 float32 的十进制表示，再精确换算为分，避免直接乘 100 截断。
	amount, ok := new(big.Rat).SetString(strconv.FormatFloat(float64(req.Amount), 'f', -1, 32))
	if !ok || amount.Sign() <= 0 {
		return status.Error(codes.InvalidArgument, "amount 须为大于 0 的金额")
	}
	amount.Mul(amount, big.NewRat(100, 1))
	if !amount.IsInt() || !amount.Num().IsInt64() {
		return status.Error(codes.InvalidArgument, "amount 最多保留两位小数，且换算为分后须在 int64 范围内")
	}
	switch TradeType(req.TradeType) {
	case JSAPI:
		if strings.TrimSpace(req.Openid) == "" || utf8.RuneCountInString(req.Openid) > 128 {
			return status.Error(codes.InvalidArgument, "JSAPI 支付须提供对应 appid 下的 openid")
		}
	case H5:
		if net.ParseIP(req.PayerClientIp) == nil {
			return status.Error(codes.InvalidArgument, "H5 支付须提供有效 payer_client_ip")
		}
		switch req.H5Type {
		case "Wap", "iOS", "Android":
		default:
			return status.Error(codes.InvalidArgument, "h5_type 须为 Wap、iOS 或 Android")
		}
	case Native, App:
	default:
		return ErrUnsupportedPayment
	}
	p.request = native.PrepayRequest{
		Appid:       core.String(p.config.AppID),
		Mchid:       core.String(p.config.MchId),
		Description: core.String(req.Description),
		OutTradeNo:  core.String(p.order.PayNo),
		NotifyUrl:   core.String(p.config.NotifyUrl),
		Amount: &native.Amount{
			Total:    core.Int64(amount.Num().Int64()),
			Currency: core.String("CNY"),
		},
	}
	if req.Attach != "" {
		p.request.Attach = core.String(req.Attach)
	}
	marshal, _ := json.Marshal(p.request)
	p.rawRequest = string(marshal)
	return nil
}

func (p *WechatPayer) JSAPI(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	r := p.request
	api := jsapi.JsapiApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, jsapi.PrepayRequest{
		Appid:       r.Appid,
		Mchid:       r.Mchid,
		Description: r.Description,
		OutTradeNo:  r.OutTradeNo,
		NotifyUrl:   r.NotifyUrl,
		Attach:      r.Attach,
		Amount: &jsapi.Amount{
			Total:    r.Amount.Total,
			Currency: r.Amount.Currency,
		},
		Payer: &jsapi.Payer{
			Openid: core.String(p.order.Request.Openid),
		},
	})
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.PrepayId == nil || *resp.PrepayId == "" {
		return status.Error(codes.Unavailable, "微信支付应答缺少 prepay_id，请用原订单号核实结果")
	}
	return p.sign(ctx, *resp.PrepayId)
}

func (p *WechatPayer) Native(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	api := native.NativeApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, p.request)
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.CodeUrl == nil || *resp.CodeUrl == "" {
		return status.Error(codes.Unavailable, "微信支付应答缺少 code_url，请用原订单号核实结果")
	}
	p.params = &paymentv1.NativePayParams{CodeUrl: *resp.CodeUrl}
	return nil
}

func (p *WechatPayer) H5(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	r := p.request
	api := h5.H5ApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, h5.PrepayRequest{
		Appid:       r.Appid,
		Mchid:       r.Mchid,
		Description: r.Description,
		OutTradeNo:  r.OutTradeNo,
		NotifyUrl:   r.NotifyUrl,
		Attach:      r.Attach,
		Amount: &h5.Amount{
			Total:    r.Amount.Total,
			Currency: r.Amount.Currency,
		},
		SceneInfo: &h5.SceneInfo{
			PayerClientIp: core.String(p.order.Request.PayerClientIp),
			H5Info: &h5.H5Info{
				Type: core.String(p.order.Request.H5Type),
			},
		},
	})
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.H5Url == nil || *resp.H5Url == "" {
		return status.Error(codes.Unavailable, "微信支付应答缺少 h5_url，请用原订单号核实结果")
	}
	p.params = &paymentv1.H5PayParams{H5Url: *resp.H5Url}
	return nil
}

func (p *WechatPayer) App(ctx context.Context) error {
	p.params = nil
	p.started = time.Now()
	r := p.request
	api := app.AppApiService{Client: p.client}
	resp, result, err := api.Prepay(ctx, app.PrepayRequest{
		Appid:       r.Appid,
		Mchid:       r.Mchid,
		Description: r.Description,
		OutTradeNo:  r.OutTradeNo,
		NotifyUrl:   r.NotifyUrl,
		Attach:      r.Attach,
		Amount: &app.Amount{
			Total:    r.Amount.Total,
			Currency: r.Amount.Currency,
		},
	})
	if err = p.finish(ctx, result, err); err != nil {
		return err
	}
	if resp == nil || resp.PrepayId == nil || *resp.PrepayId == "" {
		return status.Error(codes.Unavailable, "微信支付应答缺少 prepay_id，请用原订单号核实结果")
	}
	return p.sign(ctx, *resp.PrepayId)
}

// sign 按 JSAPI 和 APP 各自的调起支付规则签名，签名字段使用同一次生成的值。
func (p *WechatPayer) sign(ctx context.Context, prepayID string) error {
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
	signature, err := p.client.Sign(ctx, message)
	if err != nil {
		return err
	}
	if TradeType(p.order.Request.TradeType) == App {
		p.params = &paymentv1.AppPayParams{
			AppId:        p.config.AppID,
			PartnerId:    p.config.MchId,
			PrepayId:     prepayID,
			PackageValue: "Sign=WXPay",
			NonceStr:     nonce,
			TimeStamp:    timestamp,
			Sign:         signature.Signature,
		}
		return nil
	}
	p.params = &paymentv1.JSAPIPayParams{
		AppId:     p.config.AppID,
		PrepayId:  prepayID,
		TimeStamp: timestamp,
		NonceStr:  nonce,
		Package:   packageValue,
		SignType:  "RSA",
		PaySign:   signature.Signature,
	}
	return nil
}

// finish 只记录请求标识、耗时和结果，避免日志包含密钥、openid 或调起支付参数。
func (p *WechatPayer) finish(ctx context.Context, result *core.APIResult, err error) error {
	requestID, httpStatus := "", 0
	if result != nil && result.Response != nil {
		if result.Response.Body != nil {
			defer func(Body io.ReadCloser) { _ = Body.Close() }(result.Response.Body)
			body, readErr := io.ReadAll(result.Response.Body)
			p.rawResponse = string(body)
			if err == nil && readErr != nil {
				err = readErr
			}
		}
		requestID = result.Response.Header.Get("Request-ID")
		httpStatus = result.Response.StatusCode
	}
	apiCode := ""
	var apiErr *core.APIError
	if errors.As(err, &apiErr) {
		apiCode = apiErr.Code
	}
	slog.InfoContext(ctx, "wechatpay order", "out_trade_no", p.order.Request.OrderNo,
		"trade_type", p.order.Request.TradeType, "elapsed", time.Since(p.started),
		"request_id", requestID, "http_status", httpStatus, "code", apiCode,
		"success", err == nil, "error_type", fmt.Sprintf("%T", err))
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
		return status.Errorf(code, "微信支付下单返回错误码 %s", apiErr.Code)
	}
	return status.Error(codes.Unavailable, "未获得可信下单结果，请通过原商户订单号核实，重试时保持原单号和全部参数")
}

func (p *WechatPayer) GetPayResp() (*paymentv1.CreateOrderResp, error) {
	if p.params == nil {
		return nil, status.Error(codes.FailedPrecondition, "尚未取得微信支付参数")
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
