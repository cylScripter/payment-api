package pay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	paymentv1 "payment-api/api/payment/v1"
	"payment-api/internal/models"

	"github.com/shopspring/decimal"
	"github.com/smartwalle/alipay/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var alipayOrderNumber = regexp.MustCompile(`^[0-9A-Za-z_]{1,64}$`)

// AlipayPayer 保存单次下单的配置、请求和调起支付参数。
type AlipayPayer struct {
	order       *Order
	config      models.AlipayConfig
	client      *alipay.Client
	jsapiReq    alipay.TradeCreate
	nativeReq   alipay.TradePreCreate
	h5Req       alipay.TradeWapPay
	appReq      alipay.TradeAppPay
	params      proto.Message
	started     time.Time
	httpStatus  int
	rawRequest  string
	rawResponse string
}

var _ Payer = (*AlipayPayer)(nil)

func NewAlipayPayer(order *Order) (Payer, error) {
	return &AlipayPayer{order: order}, nil
}

func (p *AlipayPayer) LoadConfig(ctx context.Context) error {
	channel := p.order.ChannelConfig
	cfg, err := channel.AlipayConfig()
	if err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if channel.ChannelAppID != "" {
		cfg.AppID = channel.ChannelAppID
	}
	if channel.ChannelMerchantNo != "" {
		cfg.SellerID = channel.ChannelMerchantNo
	}
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.SellerID) == "" {
		return status.Error(codes.FailedPrecondition, "支付宝配置缺少 app_id 或 seller_id")
	}
	for name, value := range map[string]string{
		"notify_url": cfg.NotifyURL,
		"return_url": cfg.ReturnURL,
	} {
		if name == "return_url" && value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			return status.Errorf(codes.FailedPrecondition, "支付宝 %s 须为完整 HTTP 或 HTTPS 地址", name)
		}
	}
	sdkClient, err := alipay.New(cfg.AppID, cfg.PrivateKey, !cfg.Sandbox,
		alipay.WithTimeLocation(time.FixedZone("CST", 8*60*60)),
		alipay.WithHTTPClient(&http.Client{
			Timeout:   10 * time.Second,
			Transport: &alipayTransport{payer: p},
		}),
	)
	if err != nil {
		return status.Error(codes.FailedPrecondition, "解析支付宝应用私钥失败")
	}
	if err := sdkClient.LoadAliPayPublicKey(cfg.AlipayPublicKey); err != nil {
		return status.Error(codes.FailedPrecondition, "解析支付宝公钥失败")
	}
	p.config = cfg
	p.client = sdkClient
	p.order.AppID = cfg.AppID
	p.order.MerchantNo = cfg.SellerID
	p.order.NotifyURL = cfg.NotifyURL
	return nil
}

func (p *AlipayPayer) BuildReq() error {
	req := p.order.Request
	if !alipayOrderNumber.MatchString(p.order.PayNo) {
		return status.Error(codes.InvalidArgument, "支付宝支付单号须为 1～64 位字母、数字或下划线")
	}
	if strings.TrimSpace(req.Description) == "" || utf8.RuneCountInString(req.Description) > 256 {
		return status.Error(codes.InvalidArgument, "description 必填且最多 256 个字符")
	}
	if req.Amount <= 0 || math.IsNaN(float64(req.Amount)) || math.IsInf(float64(req.Amount), 0) {
		return status.Error(codes.InvalidArgument, "amount 须为大于 0 的有限金额")
	}
	amount := decimal.NewFromFloat32(req.Amount)
	if !amount.Shift(2).IsInteger() || amount.GreaterThan(decimal.NewFromInt(100000000)) {
		return status.Error(codes.InvalidArgument, "支付宝 amount 最多保留两位小数，最大为 100000000 元")
	}
	passbackParams := url.QueryEscape(req.Attach)
	if len(passbackParams) > 512 {
		return status.Error(codes.InvalidArgument, "支付宝 attach 经 URL 编码后最多 512 个字符")
	}
	trade := alipay.Trade{
		NotifyURL:      p.config.NotifyURL,
		ReturnURL:      p.config.ReturnURL,
		Subject:        req.Description,
		OutTradeNo:     p.order.PayNo,
		TotalAmount:    amount.StringFixed(2),
		SellerId:       p.config.SellerID,
		PassbackParams: passbackParams,
	}
	var request alipay.Param
	switch TradeType(req.TradeType) {
	case JSAPI:
		if strings.TrimSpace(req.Openid) == "" {
			return status.Error(codes.InvalidArgument, "支付宝 JSAPI 支付须提供当前应用下的 openid")
		}
		trade.ProductCode = "JSAPI_PAY"
		p.jsapiReq = alipay.TradeCreate{
			Trade:       trade,
			BuyerOpenId: req.Openid,
		}
		request = p.jsapiReq
	case Native:
		trade.ProductCode = "FACE_TO_FACE_PAYMENT"
		p.nativeReq = alipay.TradePreCreate{Trade: trade}
		request = p.nativeReq
	case H5:
		trade.ProductCode = "QUICK_WAP_WAY"
		p.h5Req = alipay.TradeWapPay{Trade: trade}
		request = p.h5Req
	case App:
		trade.ProductCode = "QUICK_MSECURITY_PAY"
		p.appReq = alipay.TradeAppPay{Trade: trade}
		request = p.appReq
	default:
		return ErrUnsupportedPayment
	}
	data, err := json.Marshal(map[string]any{
		"app_id":      p.config.AppID,
		"method":      request.APIName(),
		"notify_url":  p.config.NotifyURL,
		"return_url":  p.config.ReturnURL,
		"biz_content": request,
	})
	if err != nil {
		return err
	}
	p.rawRequest = string(data)
	return nil
}

func (p *AlipayPayer) JSAPI(ctx context.Context) error {
	p.params = nil
	p.rawResponse = ""
	p.started = time.Now()
	resp, err := p.client.TradeCreate(ctx, p.jsapiReq)
	if err != nil {
		return p.finish(ctx, err)
	}
	if resp == nil {
		return p.finish(ctx, errors.New("支付宝下单响应为空"))
	}
	if resp.Code != alipay.CodeSuccess {
		return p.finish(ctx, &resp.Error)
	}
	if resp.TradeNo == "" || resp.OutTradeNo != p.order.PayNo {
		return p.finish(ctx, errors.New("支付宝下单响应缺少交易号或订单号不匹配"))
	}
	p.params = &paymentv1.AlipayJSAPIPayParams{TradeNo: resp.TradeNo}
	return p.finish(ctx, nil)
}

func (p *AlipayPayer) Native(ctx context.Context) error {
	p.params = nil
	p.rawResponse = ""
	p.started = time.Now()
	resp, err := p.client.TradePreCreate(ctx, p.nativeReq)
	if err != nil {
		return p.finish(ctx, err)
	}
	if resp == nil {
		return p.finish(ctx, errors.New("支付宝下单响应为空"))
	}
	if resp.Code != alipay.CodeSuccess {
		return p.finish(ctx, &resp.Error)
	}
	if resp.QRCode == "" || resp.OutTradeNo != p.order.PayNo {
		return p.finish(ctx, errors.New("支付宝下单响应缺少二维码或订单号不匹配"))
	}
	p.params = &paymentv1.NativePayParams{CodeUrl: resp.QRCode}
	return p.finish(ctx, nil)
}

func (p *AlipayPayer) H5(ctx context.Context) error {
	p.params = nil
	p.rawResponse = ""
	p.started = time.Now()
	if ctx.Err() != nil {
		return p.finish(ctx, ctx.Err())
	}
	payURL, err := p.client.TradeWapPay(p.h5Req)
	if err != nil {
		return p.finish(ctx, err)
	}
	p.params = &paymentv1.H5PayParams{H5Url: payURL.String()}
	return p.finish(ctx, nil)
}

func (p *AlipayPayer) App(ctx context.Context) error {
	p.params = nil
	p.rawResponse = ""
	p.started = time.Now()
	if ctx.Err() != nil {
		return p.finish(ctx, ctx.Err())
	}
	orderString, err := p.client.TradeAppPay(p.appReq)
	if err != nil {
		return p.finish(ctx, err)
	}
	p.params = &paymentv1.AlipayAppPayParams{OrderString: orderString}
	return p.finish(ctx, nil)
}

func (p *AlipayPayer) finish(ctx context.Context, err error) error {
	var apiErr *alipay.Error
	errors.As(err, &apiErr)
	slog.InfoContext(ctx, "alipay order",
		"out_trade_no", p.order.PayNo,
		"trade_type", p.order.Request.TradeType,
		"elapsed", time.Since(p.started),
		"http_status", p.httpStatus,
		"success", err == nil,
		"error_type", fmt.Sprintf("%T", err),
	)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return status.FromContextError(ctx.Err()).Err()
	}
	if apiErr != nil {
		code := codes.FailedPrecondition
		switch apiErr.Code {
		case alipay.CodeUnknowError:
			code = codes.Unavailable
		case alipay.CodeMissingParam, alipay.CodeInvalidParam:
			code = codes.InvalidArgument
		case alipay.CodeCallLimited:
			code = codes.ResourceExhausted
		}
		return status.Errorf(code, "支付宝下单返回错误码 %s/%s", apiErr.Code, apiErr.SubCode)
	}
	return status.Error(codes.Unavailable, "未获得可信支付宝下单结果，请通过原支付单号核实")
}

func (p *AlipayPayer) GetPayReq() string {
	return p.rawRequest
}

// GetRawResp 返回支付宝原始响应体，H5、APP 本地生成支付参数时为空。
func (p *AlipayPayer) GetRawResp() string {
	return p.rawResponse
}

func (p *AlipayPayer) GetPayResp() (*paymentv1.CreateOrderResp, error) {
	if p.params == nil {
		return nil, status.Error(codes.FailedPrecondition, "尚未取得支付宝支付参数")
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

// alipayTransport 保存完整响应信封，交由 SDK 继续解析和验签。
type alipayTransport struct {
	payer *AlipayPayer
}

func (t *alipayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	t.payer.httpStatus = resp.StatusCode
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.payer.rawResponse = string(data)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, nil
}
